package runtime

var (
	service string
)

func Service() string {
	return service
}

func SetService(srv string) {
	service = srv
}
