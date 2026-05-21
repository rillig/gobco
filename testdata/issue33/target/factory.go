package target

// The unaliased import here has filepath.Base("iface/target") == "target",
// which equals the current package name.
// The old import-scanning logic in writeGobcoBlackBox mistook this
// for the target package's own import path.
import "github.com/rillig/gobco/testdata/issue33/iface/target"

func New() target.Adder {
	return &T{}
}
