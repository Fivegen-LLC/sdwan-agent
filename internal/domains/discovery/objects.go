package discovery

type (
	discoveryResult struct {
		Host      string
		IsPrimary bool
		Err       error
	}
)
