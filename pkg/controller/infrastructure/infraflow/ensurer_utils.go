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

type TerminalSpecMismatchError struct {
	AzureResourceIdentifier
	Offender string
	Expected string
}

func NewTerminalSpecMismatch(identifier AzureResourceIdentifier, offender, expected string) *TerminalSpecMismatchError {
	return &TerminalSpecMismatchError{identifier, offender, expected}
}

func (t *TerminalSpecMismatchError) Error() string {
	return fmt.Sprintf("differences between the current and target spec require the object to be deleted, but "+
		"the operation is not supported yet. Resource: %s, Name: %s, Offender: %s, Expected: %s", t.Kind, t.Name, t.Offender, t.Expected)
}

type TerminalConditionError struct {
	AzureResourceIdentifier
	error
}

func NewTerminalConditionError(identifier AzureResourceIdentifier, err error) *TerminalConditionError {
	return &TerminalConditionError{identifier, err}
}

func (t *TerminalConditionError) Error() string {
	return fmt.Sprintf("Unreconcilable error occured. Resource: %s, Name: %s, Error: %s", t.Kind, t.Name, t.error)
}

func (t *TerminalConditionError) Unwrap() error {
	return t.error
}

// GetObject returns the object and attempts to cast it to the specified type.
func GetObject[T any](wb shared.Whiteboard, key string) T {
	if ok := wb.HasObject(key); !ok {
		return *new(T)
	}
	o := wb.GetObject(key)
	return o.(T)
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

func ToMap[T any](arr []T, f func(T) string) map[string]T {
	res := map[string]T{}
	for _, t := range arr {
		key := f(t)
		if key == "" {
			continue
		}
		res[key] = t
	}

	return res
}
