// Copyright 2024 @proofrock
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

package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
)

// From https://raw.githubusercontent.com/monperrus/crawler-user-agents/refs/heads/master/crawler-user-agents.json

//go:embed blacklist/crawler-user-agents.json
var blacklist []byte

var crawlerNum atomic.Int32
var crawlerMap sync.Map

// Loads the file with the blacklist
func init() {
	var data []struct {
		Instances []string `json:"instances"`
	}

	if err := json.Unmarshal(blacklist, &data); err != nil {
		log.Fatal("Failed to unmarshal crawler blacklist:", err)
	}

	for _, crawler := range data {
		for _, instance := range crawler.Instances {
			crawlerMap.Store(instance, true)
			crawlerNum.Add(1)
		}
	}

	crawlerMap.Store("filewayTest", true) // For testing
}

func IsUserAgentBlacklisted(userAgent string) bool {
	_, itIs := crawlerMap.Load(userAgent)
	return itIs
}
