package dataloadgen_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vikstrous/dataloadgen"
)

type benchmarkUser struct {
	Name string
	ID   string
}

func TestUserLoader(t *testing.T) {
	ctx := context.Background()
	var fetches [][]string
	var mu sync.Mutex
	dl := dataloadgen.NewLoader(func(keys []string) ([]*benchmarkUser, []error) {
		mu.Lock()
		fetches = append(fetches, keys)
		mu.Unlock()

		users := make([]*benchmarkUser, len(keys))
		errors := make([]error, len(keys))

		for i, key := range keys {
			if strings.HasPrefix(key, "F") {
				return nil, []error{fmt.Errorf("failed all fetches")}
			}
			if strings.HasPrefix(key, "E") {
				errors[i] = fmt.Errorf("user not found")
			} else {
				users[i] = &benchmarkUser{ID: key, Name: "user " + key}
			}
		}
		return users, errors
	},
		dataloadgen.WithBatchCapacity(5),
		dataloadgen.WithWait(10*time.Millisecond),
	)

	t.Run("fetch concurrent data", func(t *testing.T) {
		t.Run("load user successfully", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load(ctx, "U1")
			if err != nil {
				t.Fatal(err)
			}
			if u.ID != "U1" {
				t.Fatal("not equal")
			}
		})

		t.Run("load failed user", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load(ctx, "E1")
			if err == nil {
				t.Fatal("error expected")
			}
			if u != nil {
				t.Fatal("not nil", u)
			}
		})

		t.Run("load many users", func(t *testing.T) {
			t.Parallel()
			u, err := dl.LoadAll(ctx, []string{"U2", "E2", "E3", "U4"})
			if u[0].Name != "user U2" {
				t.Fatal("not equal")
			}
			if u[3].Name != "user U4" {
				t.Fatal("not equal")
			}
			if err == nil {
				t.Fatal("error expected")
			}
			if err.(dataloadgen.ErrorSlice)[1] == nil {
				t.Fatal("error expected")
			}
			if err.(dataloadgen.ErrorSlice)[2] == nil {
				t.Fatal("error expected")
			}
		})

		t.Run("load thunk", func(t *testing.T) {
			t.Parallel()
			thunk1 := dl.LoadThunk(ctx, "U5")
			thunk2 := dl.LoadThunk(ctx, "E5")

			u1, err1 := thunk1()
			if err1 != nil {
				t.Fatal(err1)
			}
			if "user U5" != u1.Name {
				t.Fatal("not equal")
			}

			u2, err2 := thunk2()
			if err2 == nil {
				t.Fatal("error expected")
			}
			if u2 != nil {
				t.Fatal("not nil", u2)
			}
		})
	})

	t.Run("it sent two batches", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		if len(fetches) != 2 {
			t.Fatal("wrong length", fetches)
		}
		if len(fetches[0]) != 5 {
			t.Error("wrong number of keys in first fetch", fetches[0])
		}
		if len(fetches[1]) != 3 {
			t.Error("wrong number of keys in second fetch", fetches[0])
		}
	})

	t.Run("fetch more", func(t *testing.T) {
		t.Run("previously cached", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load(ctx, "U1")
			if err != nil {
				t.Fatal("error expected")
			}
			if u.ID != "U1" {
				t.Fatal("not equal")
			}
		})

		t.Run("load many users", func(t *testing.T) {
			t.Parallel()
			u, err := dl.LoadAll(ctx, []string{"U2", "U4"})
			if err != nil {
				t.Fatal(err)
			}
			if u[0].Name != "user U2" {
				t.Fatal("not equal")
			}
			if u[1].Name != "user U4" {
				t.Fatal("not equal")
			}
		})
	})

	t.Run("no round trips", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		if len(fetches) != 2 {
			t.Fatal("wrong length", fetches)
		}
	})

	t.Run("fetch partial", func(t *testing.T) {
		t.Run("errors not in cache cache value", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load(ctx, "E2")
			if u != nil {
				t.Fatal("not nil", u)
			}
			if err == nil {
				t.Fatal("error expected")
			}
		})

		t.Run("load all", func(t *testing.T) {
			t.Parallel()
			u, err := dl.LoadAll(ctx, []string{"U1", "U4", "E1", "U9", "U5"})
			if u[0].ID != "U1" {
				t.Fatal("not equal")
			}
			if u[1].ID != "U4" {
				t.Fatal("not equal")
			}
			if err.(dataloadgen.ErrorSlice)[2] == nil {
				t.Fatal("error expected")
			}
			if u[3].ID != "U9" {
				t.Fatal("not equal")
			}
			if u[4].ID != "U5" {
				t.Fatal("not equal")
			}
		})
	})

	t.Run("one partial trip", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		if len(fetches) != 3 {
			t.Fatal("wrong length", fetches)
		}
		// U9 only because E1 and E2 are already cached as failed and only U9 is new
		if len(fetches[2]) != 1 {
			t.Fatal("wrong length", fetches[2])
		}
	})

	t.Run("primed reads dont hit the fetcher", func(t *testing.T) {
		dl.Prime("U99", &benchmarkUser{ID: "U99", Name: "Primed user"})
		u, err := dl.Load(ctx, "U99")
		if err != nil {
			t.Fatal("error expected")
		}
		if "Primed user" != u.Name {
			t.Fatal("not equal")
		}

		if len(fetches) != 3 {
			t.Fatal("wrong length", fetches)
		}
	})

	t.Run("priming in a loop is safe", func(t *testing.T) {
		users := []benchmarkUser{
			{ID: "Alpha", Name: "Alpha"},
			{ID: "Omega", Name: "Omega"},
		}
		for _, user := range users {
			user := user
			dl.Prime(user.ID, &user)
		}

		u, err := dl.Load(ctx, "Alpha")
		if err != nil {
			t.Fatal("error expected")
		}
		if "Alpha" != u.Name {
			t.Fatal("not equal")
		}

		u, err = dl.Load(ctx, "Omega")
		if err != nil {
			t.Fatal("error expected")
		}
		if "Omega" != u.Name {
			t.Fatal("not equal")
		}

		if len(fetches) != 3 {
			t.Fatal("wrong length", fetches)
		}
	})

	t.Run("cleared results will go back to the fetcher", func(t *testing.T) {
		dl.Clear("U99")
		u, err := dl.Load(ctx, "U99")
		if err != nil {
			t.Fatal("error expected")
		}
		if "user U99" != u.Name {
			t.Fatal("not equal")
		}

		if len(fetches) != 4 {
			t.Fatal("wrong length", fetches)
		}
	})

	t.Run("load all thunk", func(t *testing.T) {
		thunk1 := dl.LoadAllThunk(ctx, []string{"U5", "U6"})
		thunk2 := dl.LoadAllThunk(ctx, []string{"U6", "E6"})

		users1, err1 := thunk1()
		if len(fetches) != 5 {
			t.Fatal("wrong length", fetches)
		}

		if err1 != nil {
			t.Fatal(err1)
		}
		if "user U5" != users1[0].Name {
			t.Fatal("not equal")
		}
		if "user U6" != users1[1].Name {
			t.Fatal("not equal")
		}

		users2, err2 := thunk2()
		// already cached
		if len(fetches) != 5 {
			t.Fatal("wrong length", fetches)
		}

		if err2.(dataloadgen.ErrorSlice)[0] != nil {
			t.Fatal(err2.(dataloadgen.ErrorSlice)[0])
		}
		if err2.(dataloadgen.ErrorSlice)[1] == nil {
			t.Fatal("error expected")
		}
		if "user U6" != users2[0].Name {
			t.Fatal("not equal")
		}
	})

	t.Run("single error return value works", func(t *testing.T) {
		user, err := dl.Load(ctx, "F1")
		if err == nil {
			t.Fatal("error expected")
		}
		if "failed all fetches" != err.Error() {
			t.Fatal("not equal")
		}
		if user != nil {
			t.Fatal("not empty", user)
		}
		if len(fetches) != 6 {
			t.Fatal("wrong length", fetches)
		}
	})

	t.Run("LoadAll does a single fetch", func(t *testing.T) {
		dl.Clear("U1")
		dl.Clear("F1")
		users, errs := dl.LoadAll(ctx, []string{"F1", "U1"})
		if len(fetches) != 7 {
			t.Fatal("wrong length", fetches)
		}
		for _, user := range users {
			if user != nil {
				t.Fatal("not empty", user)
			}
		}
		if len(errs.(dataloadgen.ErrorSlice)) != 2 {
			t.Fatal("wrong length", errs)
		}
		if errs.(dataloadgen.ErrorSlice)[0] == nil {
			t.Fatal("error expected")
		}
		if "failed all fetches" != errs.(dataloadgen.ErrorSlice)[0].Error() {
			t.Fatal("not equal")
		}
		if errs.(dataloadgen.ErrorSlice)[1] == nil {
			t.Fatal("error expected")
		}
		if "failed all fetches" != errs.(dataloadgen.ErrorSlice)[1].Error() {
			t.Fatal("not equal")
		}
	})
}
