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

// Package klog provides standard formatting to get information back to the cloud provider
package klog

import (
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

// Variable to determine which type of logging should be done
var logStdout = false

func SetOutputToStdout() {
	logStdout = true
}

// Errorf ...
func Errorf(format string, v ...interface{}) {
	if logStdout {
		fmt.Printf("ERROR: "+format+"\n", v...)
	} else {
		klog.Errorf(format, v...)
	}
}

// Infof ...
func Infof(format string, v ...interface{}) {
	if logStdout {
		timestamp := time.Now().Format("15:04:05.0000")
		fmt.Printf("INFO: ["+timestamp+"] "+format+"\n", v...)
	} else {
		klog.Infof(format, v...)
	}
}

// NotFound ...
func NotFound(format string, v ...interface{}) {
	if logStdout {
		fmt.Printf("NOT_FOUND: "+format+"\n", v...)
	} else {
		klog.Infof("NOT_FOUND: "+format, v...)
	}
}

// Pending ...
func Pending(format string, v ...interface{}) {
	if logStdout {
		fmt.Printf("PENDING: "+format+"\n", v...)
	} else {
		klog.Infof("PENDING: "+format, v...)
	}
}

// Success ...
func Success(format string, v ...interface{}) {
	if logStdout {
		fmt.Printf("SUCCESS: "+format+"\n", v...)
	} else {
		klog.Infof("SUCCESS: "+format, v...)
	}
}

// Warningf ...
func Warningf(format string, v ...interface{}) {
	if logStdout {
		fmt.Printf("WARNING: "+format+"\n", v...)
	} else {
		klog.Warningf(format, v...)
	}
}
