package relayer

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// Paths represent connection paths between chains
type Paths []*Path

// Duplicate returns true if there is a duplicate path in the array
func (p Paths) Duplicate(path *Path) bool {
	for _, pth := range p {
		if path.Equal(pth) {
			return true
		}
	}
	return false
}

// MustYAML returns the yaml string representation of the Paths
func (p Paths) MustYAML() string {
	out, err := yaml.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// MustYAML returns the yaml string representation of the Path
func (p Path) MustYAML() string {
	out, err := yaml.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// SetIndices sets the index of the path
func (p Paths) SetIndices() Paths {
	out := Paths{}
	for i, path := range p {
		foo := path
		foo.Index = i
		out = append(out, foo)
	}
	return out
}

// PathsFromChains returns a path from the config between two chains
func (p Paths) PathsFromChains(src, dst string) (Paths, error) {
	var out Paths
	for i, path := range p {
		if (path.Dst.ChainID == src || path.Src.ChainID == src) && (path.Dst.ChainID == dst || path.Src.ChainID == dst) {
			path.Index = i
			out = append(out, path)
		}
	}
	if len(out) == 0 {
		return Paths{}, fmt.Errorf("failed to find path in config between chains %s and %s", src, dst)
	}
	return out, nil
}

// Path represents a pair of chains and the identifiers needed to
// relay over them
type Path struct {
	Src      *PathEnd     `yaml:"src" json:"src"`
	Dst      *PathEnd     `yaml:"dst" json:"dst"`
	Strategy *StrategyCfg `yaml:"strategy" json:"strategy"`
	Index    int          `yaml:"index,omitempty" json:"index,omitempty"`
}

// Validate checks that a path is valid
func (p *Path) Validate() (err error) {
	if err = p.Src.Validate(FULLPATH); err != nil {
		return err
	}
	if err = p.Dst.Validate(FULLPATH); err != nil {
		return err
	}
	if _, err = p.GetStrategy(); err != nil {
		return err
	}
	return nil
}

// Equal returns true if the path ends are equivelent, false otherwise
func (p *Path) Equal(path *Path) bool {
	if (p.Src.Equal(path.Src) || p.Src.Equal(path.Dst)) && (p.Dst.Equal(path.Src) || p.Dst.Equal(path.Dst)) {
		return true
	}
	return false
}

// End returns the proper end given a chainID
func (p *Path) End(chainID string) *PathEnd {
	if p.Dst.ChainID == chainID {
		return p.Dst
	}
	if p.Src.ChainID == chainID {
		return p.Src
	}
	return &PathEnd{}
}

func (p *Path) String() string {
	return fmt.Sprintf("[%d] %s ->\n %s", p.Index, p.Src.String(), p.Dst.String())
}

// TODO: add Order chanTypes.Order as a property and wire it up in validation
// as well as in the transaction commands

// PathEnd represents the local connection identifers for a relay path
// The path is set on the chain before performing operations
type PathEnd struct {
	ChainID      string `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	ConnectionID string `yaml:"connection-id,omitempty" json:"connection-id,omitempty"`
	ChannelID    string `yaml:"channel-id,omitempty" json:"channel-id,omitempty"`
	PortID       string `yaml:"port-id,omitempty" json:"port-id,omitempty"`
}

// Equal returns true if both path ends are equivelent, false otherwise
func (p *PathEnd) Equal(path *PathEnd) bool {
	if p.ChainID == path.ChainID && p.ClientID == path.ClientID && p.ConnectionID == path.ConnectionID && p.PortID == path.PortID && p.ChannelID == path.ChannelID {
		return true
	}
	return false
}
