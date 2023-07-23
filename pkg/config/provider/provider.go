package provider

type Type byte

const (
	NilType Type = iota
	ConfigType
	FileType
)

func (t Type) String() string {
	switch t {
	case NilType:
		return "nil"
	case ConfigType:
		return "config"
	case FileType:
		return "file"
	}

	return "unknown"
}

type Data struct {
	Type   Type
	Config map[string]any
}

func (d Data) IsNil() bool {
	return d.Type == NilType || d.Config == nil
}

type Provider interface {
	Provide(chan<- Data) (Data, error)
	Close() error
}
