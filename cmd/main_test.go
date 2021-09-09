/*******************************************************************************
* IBM Cloud Kubernetes Service, 5737-D43
* (C) Copyright IBM Corp. 2021 All Rights Reserved.
*
* SPDX-License-Identifier: Apache2.0
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*    http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*******************************************************************************/

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunCmd(t *testing.T) {
	invalidOption := "Invalid option"
	invalidKubeConfig := "no such file or directory"
	requiredArgMissing := "Required argument is missing"
	tests := []struct {
		cmd       string
		wantErr   bool
		errString string
	}{
		{cmd: "help", wantErr: false},
		{cmd: "version", wantErr: false},
		{cmd: "invalid", wantErr: true, errString: invalidOption},
		{cmd: "create-lb", wantErr: true, errString: requiredArgMissing},
		{cmd: "delete-lb", wantErr: true, errString: requiredArgMissing},
		{cmd: "monitor", wantErr: true, errString: invalidKubeConfig},
		{cmd: "status-lb", wantErr: true, errString: requiredArgMissing},
		{cmd: "update-lb", wantErr: true, errString: requiredArgMissing},
	}
	os.Setenv("KUBECONFIG", "/tmp/invalid/subdir/non-existant-kubeconfig.cfg")
	for _, tt := range tests {
		t.Run("Testing "+tt.cmd+" option", func(t *testing.T) {
			err := runCmd(tt.cmd, "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("runCmd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				assert.Contains(t, err.Error(), tt.errString)
			}
		})
	}
	os.Unsetenv("KUBECONFIG")
}
