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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

const (
	// PersistentStateVersion is the current version used for persisting the state.
	PersistentStateVersion = "v1alpha1"
	// PersistentStateAPIVersion is the APIVersion used for the persistent state
	PersistentStateAPIVersion = azure.GroupName + "/" + PersistentStateVersion
	// PersistentStateKind is the kind name for the persistent state
	PersistentStateKind = "FlowState"
)

// PersistentState is the state which is persisted as part of the infrastructure status.
type PersistentState struct {
	metav1.TypeMeta

	Data map[string]string `json:"data"`
}

// NewPersistentState creates empty PersistentState
func NewPersistentState() *PersistentState {
	return &PersistentState{
		TypeMeta: metav1.TypeMeta{
			APIVersion: PersistentStateAPIVersion,
			Kind:       PersistentStateKind,
		},
		Data: map[string]string{},
	}
}

// NewPersistentStateFromJSON unmarshalls PersistentState from JSON or YAML.
// Returns nil if input contains no kind field with value "FlowState".
func NewPersistentStateFromJSON(raw []byte) (*PersistentState, error) {
	// first check if state is from flow or Terraformer
	marker := &metav1.TypeMeta{}
	if err := json.Unmarshal(raw, marker); err != nil {
		return nil, err
	}
	if marker.Kind == "" {
		// no flow state
		return nil, nil
	}
	if marker.Kind != PersistentStateKind || !strings.HasPrefix(marker.APIVersion, azure.GroupName) {
		return nil, fmt.Errorf("unknown kind or group: kind=%s, apiVersion=%s", marker.Kind, marker.APIVersion)
	}

	state := &PersistentState{}
	if err := json.Unmarshal(raw, state); err != nil {
		return nil, err
	}
	if valid, err := state.HasValidVersion(); !valid {
		return nil, err
	}

	if state.Data == nil {
		state.Data = map[string]string{}
	}
	return state, nil
}

// HasValidVersion checks if flow version is supported.
func (s *PersistentState) HasValidVersion() (valid bool, err error) {
	valid = s != nil && s.Kind == PersistentStateKind && s.APIVersion == PersistentStateAPIVersion
	if !valid {
		err = fmt.Errorf("unsupported APIVersion %s for kind %s", s.APIVersion, s.Kind)
	}
	return
}

// ToJSON marshals state as JSON
func (s *PersistentState) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}
