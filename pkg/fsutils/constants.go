package fsutils

const (
	_ = 1 << (iota * 10) // ignore the first value
	// KB ...
	KB // decimal:       1024 -> binary 00000000000000000000010000000000
	// MB ...
	MB // decimal:    1048576 -> binary 00000000000100000000000000000000
	// GB ...
	GB // decimal: 1073741824 -> binary 01000000000000000000000000000000

	//DefaultPartSize stands for the default part size when split to multipart upload
	DefaultPartSize = MB << 2
)
