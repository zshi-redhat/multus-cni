// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package types

import (
	"encoding/json"
	"fmt"

	"github.com/intel/multus-cni/logging"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
)

const (
	defaultCNIDir  = "/var/lib/cni/multus"
	defaultConfDir = "/etc/cni/multus/net.d"
)

// Convert raw CNI JSON into a DelegateNetConf structure
func LoadDelegateNetConf(bytes []byte, ifnameRequest string) (*DelegateNetConf, error) {
	delegateConf := &DelegateNetConf{}
	if err := json.Unmarshal(bytes, delegateConf); err != nil {
		return nil, fmt.Errorf("error unmarshalling delegate config: %v", err)
	}
	delegateConf.Bytes = bytes

	// Do some minimal validation
	if delegateConf.Type == "" {
		return nil, fmt.Errorf("delegate must have the 'type' field")
	}

	if ifnameRequest != "" {
		delegateConf.IfnameRequest = ifnameRequest
	}

	return delegateConf, nil
}

func LoadNetworkStatus(r types.Result, netName string, defaultNet bool) (*NetworkStatus, error) {
	// Convert whatever the IPAM result was into the current Result type
	result, err := current.NewResultFromResult(r)
	if err != nil {
		return nil, fmt.Errorf("error convert the type.Result to current.Result: %v", err)
	}

	netstatus := &NetworkStatus{}
	netstatus.Name = netName
	netstatus.Default = defaultNet

	for _, ifs := range result.Interfaces {
		//Only pod interfaces can have sandbox information
		if ifs.Sandbox != "" {
			netstatus.Interface = ifs.Name
			netstatus.Mac = ifs.Mac
		}
	}

	for _, ipconfig := range result.IPs {
		if ipconfig.Version == "4" && ipconfig.Address.IP.To4() != nil {
			netstatus.IPs = append(netstatus.IPs, ipconfig.Address.IP.String())
		}

		if ipconfig.Version == "6" && ipconfig.Address.IP.To16() != nil {
			netstatus.IPs = append(netstatus.IPs, ipconfig.Address.IP.String())
		}
	}

	netstatus.DNS = result.DNS

	return netstatus, nil

}

func LoadNetConf(bytes []byte) (*NetConf, error) {
	netconf := &NetConf{}
	if err := json.Unmarshal(bytes, netconf); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	// Logging
	if netconf.LogFile != "" {
		logging.SetLogFile(netconf.LogFile)
	}
	if netconf.LogLevel != "" {
		logging.SetLogLevel(netconf.LogLevel)
	}

	// Parse previous result
	if netconf.RawPrevResult != nil {
		resultBytes, err := json.Marshal(netconf.RawPrevResult)
		if err != nil {
			return nil, fmt.Errorf("could not serialize prevResult: %v", err)
		}
		res, err := version.NewResult(netconf.CNIVersion, resultBytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse prevResult: %v", err)
		}
		netconf.RawPrevResult = nil
		netconf.PrevResult, err = current.NewResultFromResult(res)
		if err != nil {
			return nil, fmt.Errorf("could not convert result to current version: %v", err)
		}
	}

	// Delegates must always be set. If no kubeconfig is present, the
	// delegates are executed in-order.  If a kubeconfig is present,
	// at least one delegate must be present and the first delegate is
	// the master plugin. Kubernetes CRD delegates are then appended to
	// the existing delegate list and all delegates executed in-order.

	if len(netconf.RawDelegates) == 0 {
		return nil, fmt.Errorf("at least one delegate must be specified")
	}

	if netconf.CNIDir == "" {
		netconf.CNIDir = defaultCNIDir
	}
	if netconf.ConfDir == "" {
		netconf.ConfDir = defaultConfDir
	}

	for idx, rawConf := range netconf.RawDelegates {
		bytes, err := json.Marshal(rawConf)
		if err != nil {
			return nil, fmt.Errorf("error marshalling delegate %d config: %v", idx, err)
		}
		delegateConf, err := LoadDelegateNetConf(bytes, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load delegate %d config: %v", idx, err)
		}
		netconf.Delegates = append(netconf.Delegates, delegateConf)
	}
	netconf.RawDelegates = nil

	// First delegate is always the master plugin
	netconf.Delegates[0].MasterPlugin = true

	return netconf, nil
}

// AddDelegates appends the new delegates to the delegates list
func (n *NetConf) AddDelegates(newDelegates []*DelegateNetConf) error {
	n.Delegates = append(n.Delegates, newDelegates...)
	return nil
}

// AddNetworkStatus appends the new network status to the network status list
func (n *NetConf) AddNetworkStatus(newNetStatus []*NetworkStatus) error {
	n.NetStatus = append(n.NetStatus, newNetStatus...)
	return nil
}
