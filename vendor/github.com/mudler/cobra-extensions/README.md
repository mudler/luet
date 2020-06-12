# cobra-extensions
Create git-alike extensions for your cobra projects!

## Usage

```
import "github.com/mudler/cobra-extensions"

// Detect my-awesome-cli-foo, my-awesome-cli-bar in $PATH and extensiopath1 (relative to the bin)
// it also accepts abspath
exts := extensions.Discover("my-awesome-cli", "extensiopath1", "extensiopath2")

fmt.Println("Detected extensions:", exts)

for _, ex := range exts {
  name := ex.Short()
  cobraCmd := ex.CobraCommand()
}
```
