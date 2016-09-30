# stow [![GoDoc](https://godoc.org/github.com/graymeta/stow?status.svg)](https://godoc.org/github.com/graymeta/stow)
Storage abstraction package for Go.

## How it works

Stow provides implementations for storage services, blob stores, cloud storage etc.

## Implementations

* Local (folders are containers, files are items)
* Remote (mounted) drives (NFS, CIFS, etc.)
* Amazon S3
* Google Cloud Storage
* Microsoft Azure Blob Storage
* Openstack Swift (with auth v2)
* Oracle Storage Cloud Service

## Concepts

The concepts of Stow are modelled around the most popular object storage services, and are made up of three main objects:

* `Location` - a place where many `Container` objects are stored
* `Container` - a named group of `Item` objects
* `Item` - an individual file

```
location1 (e.g. Azure)
├── container1
├───── item1.1
├───── item1.2
├───── item1.3
├── container2
├───── item2.1
├───── item2.2
location2 (e.g. local storage)
├── container1
├───── item1.1
├───── item1.2
├───── item1.3
├── container2
├───── item2.1
├───── item2.2
```

* A location contains many containers
* A container contains many items
* Containers do not contain other containers
* Items must belong to a container
* Item names may be a path

## Guides

### Walking all items

```go
kind := "s3"
config := stow.ConfigMap{
	"account-name": "stow"
	"api-key":      "abc123",
}
location, err := stow.Dial(kind, config)
if err != nil {
	return err
}
defer location.Close()
containers, _, err := location.Containers(stow.NoPrefix, stow.CursorStart, 10)
if err != nil {
	return err
}
err = stow.Walk(containers[0], stow.NoPrefix, func(item stow.Item, err error) error {
	if err != nil {
		return err
	}
	log.Println(item.Name())
	return nil
})
if err != nil {
	return err
}
```

### Getting an `Item` by URL

If you have a stow URL, you can use it to lookup the kind of location:

```go
kind, err := stow.KindByURL(url)
```

`kind` will be a string describing the kind of storage. You can then pass `kind` along with a `Config` to `stow.New` to create a new `Location` where the item for the URL is:

```go
location, err := stow.Dial(kind, config)
```

You can then get the `Item` for the specified URL from the location:

```go
item, err := location.ItemByURL(url)
```

### Cursors

Cursors are strings that provide a pointer to items in sets allowing for paging over the entire set.

Call such methods first passing in `stow.CursorStart` as the cursor, which indicates the first item/page. The method will, as one of its return arguments, provide a new cursor which you can pass into subsequent calls to the same method.

When `stow.IsCursorEnd(cursor)` returns `true`, you have reached the end of the set.
