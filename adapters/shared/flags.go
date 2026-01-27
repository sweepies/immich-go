package shared

import "github.com/spf13/pflag"

// SafeVar registers a flag only if it doesn't already exist
func SafeVar(flags *pflag.FlagSet, value pflag.Value, name string, usage string) {
	if flags.Lookup(name) == nil {
		flags.Var(value, name, usage)
	}
}

// SafeBoolVar registers a bool flag only if it doesn't already exist
func SafeBoolVar(flags *pflag.FlagSet, p *bool, name string, value bool, usage string) {
	if flags.Lookup(name) == nil {
		flags.BoolVar(p, name, value, usage)
	}
}

// SafeBoolVarP registers a bool flag with shorthand only if it doesn't already exist
func SafeBoolVarP(flags *pflag.FlagSet, p *bool, name, shorthand string, value bool, usage string) {
	if flags.Lookup(name) == nil {
		flags.BoolVarP(p, name, shorthand, value, usage)
	}
}

// SafeStringVar registers a string flag only if it doesn't already exist
func SafeStringVar(flags *pflag.FlagSet, p *string, name string, value string, usage string) {
	if flags.Lookup(name) == nil {
		flags.StringVar(p, name, value, usage)
	}
}

// SafeStringSliceVar registers a string slice flag only if it doesn't already exist
func SafeStringSliceVar(flags *pflag.FlagSet, p *[]string, name string, value []string, usage string) {
	if flags.Lookup(name) == nil {
		flags.StringSliceVar(p, name, value, usage)
	}
}

// SafeIntVar registers an int flag only if it doesn't already exist
func SafeIntVar(flags *pflag.FlagSet, p *int, name string, value int, usage string) {
	if flags.Lookup(name) == nil {
		flags.IntVar(p, name, value, usage)
	}
}
