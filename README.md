# dataloadgen

This is a fork of https://github.com/vektah/dataloaden that uses generics
instead of code genation. See the docs there for the motivation and usage details.

With this package you can skip the generation step! That's why it's dataload`gen` (gen stands for generics).

To add this package as a dependency:

```
go get github.com/vikstrous/dataloadgen
```

Here's an example constructing a loader and using it.

```go
	loader := dataloadgen.NewLoader(dataloadgen.LoaderConfig[string, int]{
		Wait:     1 * time.Millisecond,
		MaxBatch: 1,
		Fetch: func(keys []string) (ret []int, errs []error) {
			for _, key := range keys {
				num, err := strconv.ParseInt(key, 10, 32)
				ret = append(ret, int(num))
				errs = append(errs, err)
			}
			return
		},
	})
	one, _ := loader.Load("1")
	fmt.Println(one)
```