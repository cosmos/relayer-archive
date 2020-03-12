/*
Copyright © 2020 Jack Zampolin jack.zampolin@gmail.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Config represents the config file for the relayer
type Config struct {
	Global GlobalConfig  `yaml:"global" json:"global"`
	Chains ChainConfigs  `yaml:"chains" json:"chains"`
	Paths  relayer.Paths `yaml:"paths" json:"paths"`

	c relayer.Chains
}

// GlobalConfig describes any global relayer settings
type GlobalConfig struct {
	Strategy      string `yaml:"strategy" json:"strategy"`
	Timeout       string `yaml:"timeout" json:"timeout"`
	LiteCacheSize int    `yaml:"lite-cache-size" json:"lite-cache-size"`
}

// NewDefaultGlobalConfig returns a global config with defaults set
func NewDefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Strategy:      "naieve",
		Timeout:       "10s",
		LiteCacheSize: 20,
	}
}

// ChainConfigs is a collection of ChainConfig
type ChainConfigs []ChainConfig

// AddChain adds an additional chain to the config
func (c *Config) AddChain(chain ChainConfig) *Config {
	c.Chains = append(c.Chains, chain)
	return c
}

// DeleteChain removes a chain from the config
func (c *Config) DeleteChain(chain string) *Config {
	var set ChainConfigs
	for _, ch := range c.Chains {
		if ch.ChainID != chain {
			set = append(set, ch)
		}
	}
	c.Chains = set
	return c
}

// AddPath adds a path to the config file
func (c *Config) AddPath(path relayer.Path) *Config {
	c.Paths = append(c.Paths, path)
	return c
}

// DeletePath removes a path at index i
func (c *Config) DeletePath(i int) *Config {
	c.Paths = append(c.Paths[:i], c.Paths[i+1:]...)
	return c
}

// ChainConfig describes the config necessary for an individual chain
// TODO: Are there additional parameters needed here
type ChainConfig struct {
	Key            string  `yaml:"key" json:"key"`
	ChainID        string  `yaml:"chain-id" json:"chain-id"`
	RPCAddr        string  `yaml:"rpc-addr" json:"rpc-addr"`
	AccountPrefix  string  `yaml:"account-prefix" json:"account-prefix"`
	Gas            uint64  `yaml:"gas,omitempty" json:"gas,omitempty"`
	GasAdjustment  float64 `yaml:"gas-adjustment,omitempty" json:"gas-adjustment,omitempty"`
	GasPrices      string  `yaml:"gas-prices,omitempty" json:"gas-prices,omitempty"`
	DefaultDenom   string  `yaml:"default-denom,omitempty" json:"default-denom,omitempty"`
	Memo           string  `yaml:"memo,omitempty" json:"memo,omitempty"`
	TrustingPeriod string  `yaml:"trusting-period" json:"trusting-period"`
}

// Called to set the relayer.Chain types on Config
func setChains(c *Config, home string) error {
	var out []*relayer.Chain
	var new = &Config{Global: c.Global, Chains: c.Chains, Paths: c.Paths}
	for _, i := range c.Chains {
		homeDir := path.Join(home, "lite")
		chain, err := relayer.NewChain(i.Key, i.ChainID, i.RPCAddr,
			i.AccountPrefix, i.Gas, i.GasAdjustment, i.GasPrices,
			i.DefaultDenom, i.Memo, homePath, c.Global.LiteCacheSize,
			i.TrustingPeriod, homeDir, appCodec, cdc)
		if err != nil {
			return err
		}
		out = append(out, chain)
	}
	new.c = out
	config = new
	return nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command) error {
	home, err := cmd.PersistentFlags().GetString(flags.FlagHome)
	if err != nil {
		return err
	}

	config = &Config{}
	cfgPath := path.Join(home, "config", "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		viper.SetConfigFile(cfgPath)
		if err := viper.ReadInConfig(); err == nil {
			// read the config file bytes
			file, err := ioutil.ReadFile(viper.ConfigFileUsed())
			if err != nil {
				fmt.Println("Error reading file:", err)
				os.Exit(1)
			}

			// unmarshall them into the struct
			err = yaml.Unmarshal(file, config)
			if err != nil {
				fmt.Println("Error unmarshalling config:", err)
				os.Exit(1)
			}

			// ensure config has []*relayer.Chain used for all chain operations
			err = setChains(config, home)
			if err != nil {
				fmt.Println("Error parsing chain config:", err)
				os.Exit(1)
			}
		}
	}
	return nil
}

func overWriteConfig(cmd *cobra.Command, cfg *Config) error {
	home, err := cmd.Flags().GetString(flags.FlagHome)
	if err != nil {
		return err
	}

	cfgPath := path.Join(home, "config", "config.yaml")
	if _, err = os.Stat(cfgPath); err == nil {
		viper.SetConfigFile(cfgPath)
		if err = viper.ReadInConfig(); err == nil {
			// ensure setChains runs properly
			err = setChains(config, home)
			if err != nil {
				return err
			}

			// marshal the new config
			out, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}

			// overwrite the config file
			err = ioutil.WriteFile(viper.ConfigFileUsed(), out, 0666)
			if err != nil {
				return err
			}

			// set the global variable
			config = cfg
		}
	}
	return err
}
