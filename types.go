package graphql

import (
	"errors"

	"github.com/graphql-go/graphql/gqlerrors"
)

// type Schema interface{}

type Result struct {
	Data   interface{}                `json:"data"`
	Errors []gqlerrors.FormattedError `json:"errors,omitempty"`
}

func (r *Result) HasErrors() bool {
	return len(r.Errors) > 0
}

type Thunk func() (interface{}, error)

func (t Thunk) Get() (value interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			if r, ok := r.(string); ok {
				err = errors.New(r)
			}
			if r, ok := r.(error); ok {
				err = gqlerrors.FormatError(r)
			}
		}
	}()

	value, err = t()
	if err != nil {
		return
	}

	if thunk, ok := getThunk(value); ok {
		value, err = thunk.Get()
	}

	return
}

func (t Thunk) Then(next func(value interface{}) (interface{}, error)) Thunk {
	return func() (interface{}, error) {
		value, err := t.Get()
		if err != nil {
			return nil, err
		}

		return next(value)
	}
}

func (t Thunk) Catch(next func(err error) (interface{}, error)) Thunk {
	return func() (interface{}, error) {
		value, err := t.Get()
		if err != nil {
			return next(err)
		}

		return value, nil
	}
}

func WhenAll(thunksOrValues []interface{}) Thunk {
	return Thunk(func() (interface{}, error) {
		for index, value := range thunksOrValues {
			if thunk, ok := getThunk(value); ok {
				value, err := thunk.Get()

				if err != nil {
					return nil, err
				}
				thunksOrValues[index] = value
			}
		}

		return thunksOrValues, nil
	})
}

func getThunk(value interface{}) (Thunk, bool) {
	if thunk, ok := value.(Thunk); ok {
		return thunk, true
	}

	return nil, false
}

func thunkForMap(m map[string]interface{}) Thunk {
	keys := make([]string, 0, len(m))
	thunksOrValues := make([]interface{}, 0, len(m))

	for k, v := range m {
		keys = append(keys, k)
		thunksOrValues = append(thunksOrValues, v)
	}

	return WhenAll(thunksOrValues).Then(func(values interface{}) (interface{}, error) {
		for i, v := range values.([]interface{}) {
			m[keys[i]] = v
		}

		return m, nil
	})
}
