package cli

import (
	"flag"
)

var FilePath = flag.String("f", "local.yaml", "path to config file")
var Foo = flag.String("foo", "", "foo config file")
