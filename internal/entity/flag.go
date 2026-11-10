package entity

// Flag is a feature flag: its key in the provider, and the value it answers when no provider has it.
type Flag struct {
	Key     string
	Default bool
}

// FlagTaskAttributes turns task attributes on and off. On by default: it switches off a feature that
// already shipped. Owner: the task domain; remove it once attributes need no off switch.
func FlagTaskAttributes() Flag { return Flag{Key: "task-attributes", Default: true} }

// Flags lists every flag.
func Flags() []Flag { return []Flag{FlagTaskAttributes()} }

// FlagValues are the flags as one caller sees them.
type FlagValues struct {
	TaskAttributes bool `json:"taskAttributes"`
}
