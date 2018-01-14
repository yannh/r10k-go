package puppetsource

type Source interface {
	Name() string
	Remote() string
	Basedir() string
	Fetch(string) error
	Location() string
}
