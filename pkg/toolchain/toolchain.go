/*
Package toolchain provides a set of utilities to manage and use
C/C++ compilers, debuggers and memory checkers. It abstracts their
usage under convenient interfaces so tools can be interchangeably
utilized.
*/
package toolchain

import (
	"os"
	"strings"
)

func isValidImplementationName(name string) bool {
	return !strings.ContainsAny(name, string([]rune{os.PathSeparator, os.PathListSeparator}))
}

// Flags is a utility type used to manage command-line flags for executables.
type Flags map[string][]string

// Add adds a value to the specified flag.
func (f Flags) Add(flag, value string) {
	key := normalizeFlagName(flag)
	f[key] = append(f[key], value)
}

// Set sets the values for the specified flag, overriding any previous values.
// If no values are given, the flag isn't set.
func (f Flags) Set(flag string, values ...string) {
	if len(values) == 0 {
		return
	}

	key := normalizeFlagName(flag)
	f[key] = append([]string(nil), values...)
}

// Delete deletes or untoggles a flag.
func (f Flags) Delete(flag string) {
	delete(f, normalizeFlagName(flag))
}

// Toggle turns a flag on, without setting any values for it.
// Untoggle it using Del.
func (f Flags) Toggle(flag string) {
	f[normalizeFlagName(flag)] = nil
}

// Has checks whether the flag has values or is toggled.
func (f Flags) Has(flag string) bool {
	_, ok := f[normalizeFlagName(flag)]
	return ok
}

// Get the first set value of the flags. Returns the empty string if no values are set.
func (f Flags) Get(flag string) string {
	values := f.GetAll(flag)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// GetAll returns all the values for the given flag.
func (f Flags) GetAll(flag string) []string {
	return f[normalizeFlagName(flag)]
}

// Merge merges this set of flags with the provided one. Values in the other set override
// the current one's.
func (f Flags) Merge(others Flags) {
	for key, value := range others {
		f[key] = value
	}
}

// Range loops through each set flag.
func (f Flags) Range(fn func(flag string, values []string, isToggle bool)) {
	for key, value := range f {
		fn(key, value, len(value) == 0)
	}
}

func normalizeFlagName(name string) string {
	if len(name) == 0 {
		panic("toolchain: empty flag name")
	}

	if name[0] == '/' || name[0] == '-' {
		return name[1:]
	}

	return name
}
