package graphql

import (
	"errors"
	"reflect"

	"github.com/graphql-go/graphql/gqlerrors"
)

type Thunk func() (interface{}, error)

func (t Thunk) Await() (interface{}, error) {
	return t()
}

func (t Thunk) Then(handler func(value interface{}) (interface{}, error)) Thunk {
	return t.done(handler, nil)
}

func (t Thunk) Catch(handler func(err error) (interface{}, error)) Thunk {
	return t.done(nil, handler)
}

func (t Thunk) done(
	successHandler func(value interface{}) (interface{}, error),
	errorHandler func(err error) (interface{}, error),
) Thunk {
	return func() (value interface{}, err error) {
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

		value, err = t.Await()
		if err != nil && errorHandler != nil {
			value, err = errorHandler(err)
		} else if successHandler != nil {
			value, err = successHandler(value)
		} else {
			return
		}

		// If one of the handlers returned another thunk, resolve it
		// so we don't end up with nested thunks
		if thunk, ok := getThunk(value); ok {
			value, err = thunk.Await()
		}

		return
	}
}

func WhenAll(thunksOrValues []interface{}) Thunk {
	return Thunk(func() (interface{}, error) {
		for index, value := range thunksOrValues {
			if thunk, ok := getThunk(value); ok {
				value, err := thunk.Await()

				if err != nil {
					return nil, err
				}
				thunksOrValues[index] = value
			}
		}

		return thunksOrValues, nil
	})
}

func getThunk(value interface{}) (thunk Thunk, ok bool) {
	if value == nil {
		return
	}

	if thunk, ok = value.(Thunk); ok {
		return
	}

	if reflect.TypeOf(value).Kind() == reflect.Func &&
		reflect.TypeOf(value).ConvertibleTo(reflect.TypeOf(thunk)) {
		thunk, ok = reflect.ValueOf(value).Convert(reflect.TypeOf(thunk)).Interface().(Thunk)
	}

	return
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
