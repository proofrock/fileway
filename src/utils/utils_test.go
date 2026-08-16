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

package utils

import (
	"os"
	"testing"
)

func TestHumanReadableSize(t *testing.T) {
	cases := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
		{1099511627776, "1.0 TB"},
		{1125899906842624, "1.0 PB"},
		{1152921504606846976, "1.0 EB"},
	}
	for _, c := range cases {
		got := HumanReadableSize(c.input)
		if got != c.want {
			t.Errorf("HumanReadableSize(%d) = %q, I want %q", c.input, got, c.want)
		}
	}
}

func TestGetIntEnv(t *testing.T) {
	const name = "FILEWAY_TEST_INT"

	os.Unsetenv(name)
	if got := GetIntEnv(name, 42); got != 42 {
		t.Errorf("unset -> %d, I want the default 42", got)
	}

	os.Setenv(name, "")
	defer os.Unsetenv(name)
	if got := GetIntEnv(name, 42); got != 42 {
		t.Errorf("empty -> %d, I want the default 42", got)
	}

	os.Setenv(name, "7")
	if got := GetIntEnv(name, 42); got != 7 {
		t.Errorf(`"7" -> %d, I want 7`, got)
	}

	os.Setenv(name, "-3")
	if got := GetIntEnv(name, 42); got != -3 {
		t.Errorf(`"-3" -> %d, I want -3 (range is the caller's business)`, got)
	}
}
