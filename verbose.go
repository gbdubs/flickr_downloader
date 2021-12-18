package flickr_downloader

import "fmt"

func verbose(verboseIndent int, format string, args ...interface{}) {
	if verboseIndent == -1 {
		return
	}
	s := ""
	for i := 0; i < verboseIndent; i++ {
		s += "  "
	}
	fmt.Printf(s+format, args...)
}

func (i *Input) verbose(format string, args ...interface{}) {
	verbose(i.VerboseIndent, format, args...)
}
