package domain

type GitignoreStatus int

const (
	GitignoreUnknown GitignoreStatus = iota
	GitignoreIgnored
	GitignoreNotIgnored
)

func (s GitignoreStatus) Known() bool {
	return s == GitignoreIgnored || s == GitignoreNotIgnored
}

func (s GitignoreStatus) String() string {
	switch s {
	case GitignoreIgnored:
		return "ignored"
	case GitignoreNotIgnored:
		return "not-ignored"
	default:
		return "unknown"
	}
}

func (s GitignoreStatus) MarshalJSON() ([]byte, error) {
	switch s {
	case GitignoreIgnored:
		return []byte("true"), nil
	case GitignoreNotIgnored:
		return []byte("false"), nil
	default:
		return []byte("null"), nil
	}
}

type Target struct {
	Path       string
	Exists     bool
	Gitignored GitignoreStatus
}
