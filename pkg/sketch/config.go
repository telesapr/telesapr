package sketch

import (
	"fmt"
	"maps"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// A configuration (roadmap 16.2, A2) is a named set of the parameters'
// expressions: the same model small and large, by one parameter table
// filled two ways. One may be active. Making one active writes its
// expressions into the table, and while it is active, a change to the
// table is a change to it; with none active, the table is changed
// alone. So the table always holds what the model is made from, and
// configurations only fill it.

// Configuration is a named set of parameter expressions, by name.
type Configuration struct {
	Name  string
	Exprs map[string]string
}

// cloneConfigurations copies configurations deep, for undo.
func cloneConfigurations(cs []Configuration) []Configuration {
	if cs == nil {
		return nil
	}
	out := make([]Configuration, len(cs))
	for i, c := range cs {
		out[i] = Configuration{Name: c.Name, Exprs: maps.Clone(c.Exprs)}
	}
	return out
}

// Configuration is the configuration called name, or nil.
func (sk *Sketch) Configuration(name string) *Configuration {
	for i := range sk.Configurations {
		if sk.Configurations[i].Name == name {
			return &sk.Configurations[i]
		}
	}
	return nil
}

// activeConfiguration is the active configuration, or nil.
func (sk *Sketch) activeConfiguration() *Configuration {
	if sk.ActiveConfiguration == "" {
		return nil
	}
	return sk.Configuration(sk.ActiveConfiguration)
}

// checkConfigurationName refuses an empty name, or one taken.
func (sk *Sketch) checkConfigurationName(name string) error {
	if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) {
		return fmt.Errorf("%q can't name a configuration: give it a name without spaces at either end", name)
	}
	if sk.Configuration(name) != nil {
		return fmt.Errorf("there is already a configuration %q", name)
	}
	return nil
}

// AddConfiguration adds a configuration of the table's expressions as
// they are, and makes it active.
func (sk *Sketch) AddConfiguration(name string) error {
	if err := sk.checkConfigurationName(name); err != nil {
		return err
	}
	c := Configuration{Name: name, Exprs: map[string]string{}}
	for _, p := range sk.Parameters {
		c.Exprs[p.Name] = p.Expr
	}
	sk.Configurations = append(sk.Configurations, c)
	sk.ActiveConfiguration = name
	return nil
}

// ActivateConfiguration makes the configuration called name active,
// writing its expressions into the table, or with "" none; the table
// stays as it is then.
func (sk *Sketch) ActivateConfiguration(name string) error {
	if name == "" {
		sk.ActiveConfiguration = ""
		return nil
	}
	c := sk.Configuration(name)
	if c == nil {
		return fmt.Errorf("no configuration %q", name)
	}
	old := append([]Parameter(nil), sk.Parameters...)
	for i := range sk.Parameters {
		if e, ok := c.Exprs[sk.Parameters[i].Name]; ok {
			sk.Parameters[i].Expr = e
		}
	}
	sk.applyParameters()
	for _, p := range sk.Parameters {
		if p.Err != "" {
			err := fmt.Errorf("in configuration %q, parameter %s: %s", name, p.Name, p.Err)
			sk.Parameters = old
			sk.applyParameters()
			return err
		}
	}
	sk.ActiveConfiguration = name
	sk.markAllDirty()
	return nil
}

// markAllDirty regenerates every group: a configuration may change any
// value.
func (sk *Sketch) markAllDirty() {
	if len(sk.GroupOrder) > 0 {
		sk.MarkGroupDirty(sk.GroupOrder[0], false)
	}
}

// RenameConfiguration renames a configuration.
func (sk *Sketch) RenameConfiguration(from, to string) error {
	c := sk.Configuration(from)
	if c == nil {
		return fmt.Errorf("no configuration %q", from)
	}
	if from == to {
		return nil
	}
	if err := sk.checkConfigurationName(to); err != nil {
		return err
	}
	c.Name = to
	if sk.ActiveConfiguration == from {
		sk.ActiveConfiguration = to
	}
	return nil
}

// DeleteConfiguration removes a configuration; the table stays as it
// is.
func (sk *Sketch) DeleteConfiguration(name string) error {
	for i := range sk.Configurations {
		if sk.Configurations[i].Name == name {
			sk.Configurations = append(sk.Configurations[:i], sk.Configurations[i+1:]...)
			if sk.ActiveConfiguration == name {
				sk.ActiveConfiguration = ""
			}
			return nil
		}
	}
	return fmt.Errorf("no configuration %q", name)
}

// configParameterSet keeps the active configuration as the table after
// a parameter is added or given a new expression.
func (sk *Sketch) configParameterSet(name, expr string) {
	if c := sk.activeConfiguration(); c != nil {
		c.Exprs[name] = expr
	}
}

// configParameterRenamed renames a parameter in every configuration,
// its key and its uses.
func (sk *Sketch) configParameterRenamed(from, to string) {
	for i := range sk.Configurations {
		c := &sk.Configurations[i]
		next := make(map[string]string, len(c.Exprs))
		for k, e := range c.Exprs {
			if k == from {
				k = to
			}
			next[k] = core.RenameVariable(e, from, to)
		}
		c.Exprs = next
	}
}

// configParameterDeleted takes a parameter out of every configuration.
func (sk *Sketch) configParameterDeleted(name string) {
	for i := range sk.Configurations {
		delete(sk.Configurations[i].Exprs, name)
	}
}

// configurationUsers are the configurations whose expressions name the
// parameter, as ParameterUsers describes them.
func (sk *Sketch) configurationUsers(name string) []string {
	var users []string
	for _, c := range sk.Configurations {
		// The active one is the table, whose users are counted apart.
		if c.Name == sk.ActiveConfiguration {
			continue
		}
		for k, e := range c.Exprs {
			if k == name {
				continue
			}
			if x, err := core.NewFromString(e); err == nil {
				for _, n := range x.Variables() {
					if n == name {
						users = append(users, fmt.Sprintf("configuration %s (parameter %s)", c.Name, k))
					}
				}
			}
		}
	}
	return users
}
