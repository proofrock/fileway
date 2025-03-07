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
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func genRandomString(length int) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		result[i] = alphabet[n.Int64()]
	}
	return string(result)
}

func getIntEnv(name string, deflt int) int {
	if val, isthere := os.LookupEnv(name); !isthere || val == "" {
		return deflt
	} else if ret, err := strconv.Atoi(val); err != nil {
		return deflt
	} else {
		return ret
	}
}

func replace(src []byte, toreplace, replacer string) []byte {
	ret := string(src)
	ret = strings.ReplaceAll(ret, toreplace, replacer)
	return []byte(ret)
}

func humanReadableSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}

func nowString() string {
	return time.Now().Format("20060102_150405")
}
