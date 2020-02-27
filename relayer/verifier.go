package relayer

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	dbm "github.com/tendermint/tm-db"
	"golang.org/x/sync/errgroup"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	lite "github.com/tendermint/tendermint/lite2"
	litep "github.com/tendermint/tendermint/lite2/provider"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// StartUpdatingLiteClient begins a loop that periodically updates the lite
// database.
func (c *Chain) StartUpdatingLiteClient(period time.Duration) {
	ticker := time.NewTicker(period)
	for ; true; <-ticker.C {
		err := c.UpdateLiteDBToLatestHeader()
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}

// UpdateLiteWithHeader calls UpdateLiteDbToLatestHeader and then
// GetLatestLiteHeader.
func (c *Chain) UpdateLiteWithHeader() (*tmclient.Header, error) {
	err := c.UpdateLiteDBToLatestHeader()
	if err != nil {
		return nil, err
	}
	return c.GetLatestLiteHeader()
}

// UpdatesWithHeaders calls UpdateLiteDBsToLatestHeaders then GetLatestHeaders.
func UpdatesWithHeaders(chains ...*Chain) (map[string]*tmclient.Header, error) {
	if err := UpdateLiteDBsToLatestHeaders(chains...); err != nil {
		return nil, err
	}
	return GetLatestHeaders(chains...)
}

// UpdateLiteDBToLatestHeader spins up an instance of the lite client as part
// of the chain.
func (c *Chain) UpdateLiteDBToLatestHeader() error {
	// create database connection
	db, df, err := c.NewLiteDB()
	if err != nil {
		return err
	}
	defer df()

	// initialise lite client
	lc, err := c.InitLiteClientWithoutTrust(db)
	if err != nil {
		return err
	}

	now := time.Now()

	// remove expired headers
	lc.RemoveNoLongerTrustedHeaders(now)

	// sync lite client to the most recent header of the primary provider
	return lc.Update(now)

}

// UpdateLiteDBsToLatestHeaders updates the light clients of the given chains
// to the latest state.
func UpdateLiteDBsToLatestHeaders(chains ...*Chain) error {
	var g errgroup.Group

	for _, chain := range chains {
		chain := chain
		g.Go(func() error {
			if err := chain.UpdateLiteDBToLatestHeader(); err != nil {
				return fmt.Errorf("failed to update chain %s: %w", chain, err)
			}
			return nil
		})
	}

	return g.Wait()
}

// InitLiteClientWithoutTrust reads the trusted period off of the chain.
func (c *Chain) InitLiteClientWithoutTrust(db *dbm.GoLevelDB) (*lite.Client, error) {
	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return nil, err
	}

	// TODO: provide actual witnesses!
	lc, err := lite.NewClientFromTrustedStore(c.ChainID, c.TrustingPeriod, httpProvider,
		[]litep.Provider{httpProvider}, dbs.New(db, ""),
		lite.Logger(log.NewTMLogger(log.NewSyncWriter(ioutil.Discard))))
	if err != nil {
		return nil, err
	}

	err = lc.Update(time.Now())
	if err != nil {
		return nil, err
	}

	return lc, nil
}

// InitLiteClient initializes the lite client for a given chain.
func (c *Chain) InitLiteClient(db *dbm.GoLevelDB, trustOpts lite.TrustOptions) (*lite.Client, error) {
	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return nil, err
	}

	// TODO: provide actual witnesses!
	lc, err := lite.NewClient(c.ChainID, trustOpts, httpProvider,
		[]litep.Provider{httpProvider}, dbs.New(db, ""),
		lite.Logger(log.NewTMLogger(log.NewSyncWriter(os.Stdout))))
	if err != nil {
		return nil, err
	}

	err = lc.Update(time.Now())
	if err != nil {
		return nil, err
	}

	return lc, nil
}

// TrustNodeInitClient trusts the configured node and initializes the lite client
func (c *Chain) TrustNodeInitClient(db *dbm.GoLevelDB) (*lite.Client, error) {
	// fetch latest height from configured node
	height, err := c.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	// fetch header from configured node
	header, err := c.QueryHeaderAtHeight(height)
	if err != nil {
		return nil, err
	}

	// initialize the lite client database
	out, err := c.InitLiteClient(db, c.TrustOptions(height, header.Hash().Bytes()))
	if err != nil {
		return nil, err
	}

	return out, nil
}

// NewLiteDB returns a new instance of the liteclient database connection
// CONTRACT: must close the database connection when done with it (defer df())
func (c *Chain) NewLiteDB() (db *dbm.GoLevelDB, df func(), err error) {
	db, err = dbm.NewGoLevelDB(c.ChainID, liteDir(c.HomePath))
	df = func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("can't open lite client database: %w", err)
	}
	return
}

// DeleteLiteDB removes the lite client database on disk, forcing re-initialization
func (c *Chain) DeleteLiteDB() error {
	return os.RemoveAll(filepath.Join(liteDir(c.HomePath), fmt.Sprintf("%s.db", c.ChainID)))
}

// TrustOptions returns lite.TrustOptions given a height and hash
func (c *Chain) TrustOptions(height int64, hash []byte) lite.TrustOptions {
	return lite.TrustOptions{
		Period: c.TrustingPeriod,
		Height: height,
		Hash:   hash,
	}
}

// GetLatestLiteHeader returns the header to be used for client creation
func (c *Chain) GetLatestLiteHeader() (*tmclient.Header, error) {
	height, err := c.GetLatestLiteHeight()
	if err != nil {
		return nil, err
	}
	if height == -1 {
		return nil, ErrLiteNotInitialized
	}
	return c.GetLiteSignedHeaderAtHeight(height)
}

// GetLatestHeaders gets latest trusted headers for the given chains from the
// light clients. It returns a map chainID => Header.
func GetLatestHeaders(chains ...*Chain) (map[string]*tmclient.Header, error) {
	var (
		g       errgroup.Group
		headers = make(chan *tmclient.Header, len(chains))
	)
	for _, chain := range chains {
		chain := chain
		g.Go(func() error {
			h, err := chain.GetLatestLiteHeader()
			if err != nil {
				return fmt.Errorf("failed to get latest header for chain %s: %w", chain, err)
			}
			headers <- h
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	m := make(map[string]*tmclient.Header, len(chains))
	for h := range headers {
		m[h.ChainID] = h
	}
	close(headers)

	return m, nil
}

// VerifyProof performs response proof verification.
func (c *Chain) VerifyProof(queryPath string, resp abci.ResponseQuery) error {
	// TODO: write this verify function
	return nil
}

// ValidateTxResult takes a transaction and validates the proof against a stored root of trust
func (c *Chain) ValidateTxResult(resTx *ctypes.ResultTx) (err error) {
	// fetch the header at the height from the ResultTx from the lite database
	check, err := c.GetLiteSignedHeaderAtHeight(resTx.Height)
	if err != nil {
		return
	}

	// validate the proof against that header
	err = resTx.Proof.Validate(check.Header.DataHash)
	if err != nil {
		return
	}

	return
}

// GetLatestLiteHeight uses the CLI utilities to pull the latest height from a given chain
func (c *Chain) GetLatestLiteHeight() (int64, error) {
	db, df, err := c.NewLiteDB()
	if err != nil {
		return -1, err
	}
	defer df()

	client, err := c.InitLiteClientWithoutTrust(db)
	if err != nil {
		return -1, err
	}

	return client.LastTrustedHeight()
}

// GetLatestHeights returns the latest heights from the database
func GetLatestHeights(chains ...*Chain) (map[string]int64, error) {
	hs := &heights{Map: make(map[string]int64), Errs: []error{}}
	var wg sync.WaitGroup
	for _, chain := range chains {
		wg.Add(1)
		go func(hs *heights, wg *sync.WaitGroup, chain *Chain) {
			height, err := chain.GetLatestLiteHeight()
			if err != nil {
				hs.Lock()
				hs.Errs = append(hs.Errs, err)
				hs.Unlock()
			}
			hs.Lock()
			hs.Map[chain.ChainID] = height
			hs.Unlock()
			wg.Done()
		}(hs, &wg, chain)
	}
	wg.Wait()
	return hs.out(), hs.err()
}

// GetLiteSignedHeaderAtHeight returns a signed header at a particular height.
func (c *Chain) GetLiteSignedHeaderAtHeight(height int64) (*tmclient.Header, error) {
	// create database connection
	db, df, err := c.NewLiteDB()
	if err != nil {
		return nil, err
	}
	defer df()

	client, err := c.InitLiteClientWithoutTrust(db)
	if err != nil {
		return nil, err
	}

	sh, err := client.TrustedHeader(height, time.Now())
	if err != nil {
		return nil, err
	}

	vs, err := client.TrustedValidatorSet(height, time.Now())
	if err != nil {
		return nil, err
	}

	// NOTE: TrustedValSet takes the height and subtracts 1 so this _should_
	// return the valset from height
	nvs, err := client.TrustedValidatorSet(height+1, time.Now())
	if err != nil {
		return nil, err
	}

	header := tmclient.Header{SignedHeader: *sh, ValidatorSet: vs, NextValidatorSet: nvs}
	if err = header.ValidateBasic(c.ChainID); err != nil {
		panic(fmt.Sprintf("header failed ValidateBasic: %s", err))
	}

	return &header, nil
}

// ErrLiteNotInitialized returns the cannonical error for a an uninitialized lite client
var ErrLiteNotInitialized = errors.New("lite client is not initialized")
