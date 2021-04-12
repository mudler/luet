package compression

type Implementation string

const (
	None      Implementation = "none" // e.g. tar for standard packages
	GZip      Implementation = "gzip"
	Zstandard Implementation = "zstd"
)
