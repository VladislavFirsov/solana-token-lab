package domain

// Source represents the discovery source type for a token candidate.
type Source string

const (
	SourceNewToken    Source = "NEW_TOKEN"
	SourceActiveToken Source = "ACTIVE_TOKEN"
)

// String returns the string representation of Source.
func (s Source) String() string {
	return string(s)
}

// IsValid checks if the source is a valid value.
func (s Source) IsValid() bool {
	return s == SourceNewToken || s == SourceActiveToken
}
