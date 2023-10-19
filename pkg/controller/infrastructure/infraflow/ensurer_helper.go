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

func ToList[T comparable, Y any](m map[T]Y) []Y {
	res := make([]Y, 0, len(m))
	for _, v := range m {
		res = append(res, v)
	}

	return res
}

func CopyMap[T comparable, Y any](src map[T]Y) map[T]Y {
	if src == nil {
		return nil
	}
	dst := map[T]Y{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

type Identifier interface {
	GetId() string
	GetOwnerId() string
}

type SimpleInventory[T Identifier] struct {
	inventory map[string]T
	byOwner   map[string]map[string]T
}

func NewSimpleInventory[T Identifier]() *SimpleInventory[T] {
	return &SimpleInventory[T]{
		inventory: make(map[string]T),
		byOwner:   make(map[string]map[string]T),
	}
}

func (i *SimpleInventory[T]) Insert(t T) {
	i.inventory[t.GetId()] = t

	if pid := t.GetOwnerId(); len(pid) > 0 {
		if _, ok := i.byOwner[pid]; !ok {
			i.byOwner[pid] = make(map[string]T)
		}
		i.byOwner[pid][t.GetId()] = t
	}
}

func (i *SimpleInventory[T]) Delete(t T) {
	delete(i.inventory, t.GetId())

	if pid := t.GetOwnerId(); len(pid) > 0 {
		delete(i.byOwner[pid], t.GetId())
	}

	if _, ok := i.byOwner[t.GetId()]; ok {
		delete(i.byOwner, t.GetId())
	}
}

func (i *SimpleInventory[T]) ToList() []T {
	return ToList(i.inventory)
}
