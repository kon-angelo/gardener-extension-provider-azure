//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package infraflow

import (
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// GetObject returns the object and attempts to cast it to the specified type.
func GetObject[T any](wb shared.Whiteboard, key string) T {
	if ok := wb.HasObject(key); !ok {
		return *new(T)
	}
	o := wb.GetObject(key)
	return o.(T)
}

// EnsureObjectKeys returns an error if the provided keys do not exist.
func EnsureObjectKeys(wb shared.Whiteboard, keys ...string) error {
	for _, k := range keys {
		if wb.GetObject(k) == nil {
			return fmt.Errorf("could not locate required key: %s", k)
		}
	}
	return nil
}

func Apply[T any](t T, funcs ...func(T) T) T {
	for _, f := range funcs {
		t = f(t)
	}
	return t
}

func Filter[T any](arr []T, fs ...func(T) bool) []T {
	var res []T
	for _, t := range arr {
		func() {
			for _, f := range fs {
				if !f(t) {
					return
				}
			}
			res = append(res, t)
		}()
	}
	return res
}

// ToMap converts an array into a map. The key is provided by applying "f" to the array objects and the value are the objects.
func ToMap[T comparable, Y comparable](arr []T, f func(T) Y) map[Y]T {
	res := map[Y]T{}
	for _, t := range arr {
		key := f(t)
		// if key is with default value e.g. ""
		if key == *(new(Y)) {
			continue
		}
		res[key] = t
	}

	return res
}

// Join merges maps by appending m2 to m1.
func Join[K comparable, V any](m1, m2 map[K]V) map[K]V {
	if m2 == nil {
		return m1
	}
	if m1 == nil {
		m1 = make(map[K]V)
	}

	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

// func JoinMapSlice[T any](m1, m2 map[string][]T) map[string][]T {
// 	res := make(map[string][]T)
// 	for k, v := range m1 {
// 		res[k] = append(make([]T, len(v)), v...)
// 		if add, ok := m2[k]; ok {
// 			res[k] = append(res[k], add...)
// 		}
// 	}
// 	for k, v := range m2 {
// 		if _, ok := m1[k]; !ok {
// 			res[k] = append(make([]T, len(v)), v...)
// 		}
// 	}
// 	return res
// }
