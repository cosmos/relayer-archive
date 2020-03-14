package relayer 

import (
	"fmt"
	host "github.com/cosmos/cosmos-sdk/x/ibc/24-host"
)

// Vclient validates the client identifer in the path
func (p *PathEnd) Vclient() error {
	return host.DefaultClientIdentifierValidator(p.ClientID)
}

// Vconn validates the connection identifer in the path
func (p *PathEnd) Vconn() error {
	return host.DefaultConnectionIdentifierValidator(p.ConnectionID)
}

// Vchan validates the channel identifer in the path
func (p *PathEnd) Vchan() error {
	return host.DefaultChannelIdentifierValidator(p.ChannelID)
}

// Vport validates the port identifer in the path
func (p *PathEnd) Vport() error {
	return host.DefaultPortIdentifierValidator(p.PortID)
}

func (p PathEnd) String() string {
	return fmt.Sprintf("client{%s}-conn{%s}-chan{%s}@chain{%s}:port{%s}", p.ClientID, p.ConnectionID, p.ChannelID, p.ChainID, p.PortID)
}

// pathType helps define what validations need to be run
type pathType byte

const (
	// CLNTPATH sets chainID + clientID
	CLNTPATH pathType = iota

	// CONNPATH sets CLNTPATH + connectionID
	CONNPATH

	// CHANPATH sets channelID + portID
	CHANPATH

	// CLNTCHANPATH sets CLNTPATH + CHANPATH
	CLNTCHANPATH

	// FULLPATH sets all fields
	FULLPATH
)

func (p pathType) String() string {
	switch p {
	case CLNTPATH:
		return "CLNTPATH"
	case CONNPATH:
		return "CONNPATH"
	case CHANPATH:
		return "CHANPATH"
	case CLNTCHANPATH:
		return "CLNTCHANPATH"
	case FULLPATH:
		return "FULLPATH"
	default:
		return "shouldn't be here"
	}
}

// PathClient used to set the path for client creation commands
func (c *Chain) PathClient(clientID string) error {
	return c.SetPath(&PathEnd{
		ChainID:  c.ChainID,
		ClientID: clientID,
	}, CLNTPATH)
}

// PathConnection used to set the path for the connection creation commands
func (c *Chain) PathConnection(clientID, connectionID string) error {
	return c.SetPath(&PathEnd{
		ChainID:      c.ChainID,
		ClientID:     clientID,
		ConnectionID: connectionID,
	}, CONNPATH)
}

// PathChannel used to set the path for the channel creation commands
func (c *Chain) PathChannel(channelID, portID string) error {
	return c.SetPath(&PathEnd{
		ChainID:   c.ChainID,
		ChannelID: channelID,
		PortID:    portID,
	}, CHANPATH)
}

// PathChannelClient used to set the path for channel and client commands
func (c *Chain) PathChannelClient(clientID, channelID, portID string) error {
	return c.SetPath(&PathEnd{
		ChainID:   c.ChainID,
		ClientID:  clientID,
		ChannelID: channelID,
		PortID:    portID,
	}, CLNTCHANPATH)
}

// FullPath sets all of the properties on the path
func (c *Chain) FullPath(clientID, connectionID, channelID, portID string) error {
	return c.SetPath(&PathEnd{
		ChainID:      c.ChainID,
		ClientID:     clientID,
		ConnectionID: connectionID,
		ChannelID:    channelID,
		PortID:       portID,
	}, FULLPATH)
}

// PathSet check if the chain has a path set
func (c *Chain) PathSet() bool {
	if c.PathEnd == nil {
		return false
	}
	return true
}

// PathsSet checks if the chains have their paths set
func PathsSet(chains ...*Chain) bool {
	for _, c := range chains {
		if !c.PathSet() {
			return false
		}
	}
	return true
}

// SetPath sets the path and validates the identifiers
func (c *Chain) SetPath(p *PathEnd, t pathType) error {
	err := p.Validate(t)
	if err != nil {
		return err
	}
	c.PathEnd = p
	return nil
}

// Validate returns errors about invalid identifiers as well as
// unset path variables for the appropriate type
func (p *PathEnd) Validate(t pathType) error {
	switch t {
	case CLNTPATH:
		if err := p.Vclient(); err != nil {
			return err
		}
		return nil
	case CONNPATH:
		if err := p.Vclient(); err != nil {
			return err
		}
		if err := p.Vconn(); err != nil {
			return err
		}
		return nil
	case CHANPATH:
		if err := p.Vchan(); err != nil {
			return err
		}
		if err := p.Vport(); err != nil {
			return err
		}
		return nil
	case CLNTCHANPATH:
		if err := p.Vclient(); err != nil {
			return err
		}
		if err := p.Vchan(); err != nil {
			return err
		}
		if err := p.Vport(); err != nil {
			return err
		}
		return nil
	case FULLPATH:
		if err := p.Vclient(); err != nil {
			return err
		}
		if err := p.Vconn(); err != nil {
			return err
		}
		if err := p.Vchan(); err != nil {
			return err
		}
		if err := p.Vport(); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid path type: %s", t.String())
	}
}

// ErrPathNotSet returns information what identifiers are needed to relay
func (c *Chain) ErrPathNotSet(t pathType, err error) error {
	return fmt.Errorf("Path of type %s on chain %s not set: %w", t.String(), c.ChainID, err)
}

// ErrCantSetPath returns an error if the path doesn't set properly
func (c *Chain) ErrCantSetPath(t pathType, err error) error {
	return fmt.Errorf("Path of type %s on chain %s failed to set: %w", t.String(), c.ChainID, err)
}
