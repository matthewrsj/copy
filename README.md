# copy

copy is a go package that allows one to recursively copy files, directories, or
links. It also allows the user to attempt a hardlink before falling back to an
expensive recursive copy if the link fails. A link is much faster than a
recursive copy, but links will fail if one tries to cross a partition boundary.

```golang
import "github.com/matthewrsj/copy"

// recursive copy of src to dst
err := copy.All("path/to/src", "path/to/dst")

// attempt to link first but fall back to copy if the
// link fails. Useful for crossing partition boundaries.
err := copy.LinkOrCopy("path/to/src", "path/to/dst")
```

### Resources

* https://blog.depado.eu/post/copy-files-and-directories-in-go
* https://github.com/otiai10/copy
