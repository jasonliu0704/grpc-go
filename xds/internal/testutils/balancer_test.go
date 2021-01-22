/*
 *
 * Copyright 2020 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package testutils

import (
	"google.golang.org/grpc/balancer/apis"
	"testing"
)

func TestIsRoundRobin(t *testing.T) {
	var (
		sc1 = TestSubConns[0]
		sc2 = TestSubConns[1]
		sc3 = TestSubConns[2]
	)

	testCases := []struct {
		desc string
		want []apis.SubConn
		got  []apis.SubConn
		pass bool
	}{
		{
			desc: "0 element",
			want: []apis.SubConn{},
			got:  []apis.SubConn{},
			pass: true,
		},
		{
			desc: "1 element RR",
			want: []apis.SubConn{sc1},
			got:  []apis.SubConn{sc1, sc1, sc1, sc1},
			pass: true,
		},
		{
			desc: "1 element not RR",
			want: []apis.SubConn{sc1},
			got:  []apis.SubConn{sc1, sc2, sc1},
			pass: false,
		},
		{
			desc: "2 elements RR",
			want: []apis.SubConn{sc1, sc2},
			got:  []apis.SubConn{sc1, sc2, sc1, sc2, sc1, sc2},
			pass: true,
		},
		{
			desc: "2 elements RR different order from want",
			want: []apis.SubConn{sc2, sc1},
			got:  []apis.SubConn{sc1, sc2, sc1, sc2, sc1, sc2},
			pass: true,
		},
		{
			desc: "2 elements RR not RR, mistake in first iter",
			want: []apis.SubConn{sc1, sc2},
			got:  []apis.SubConn{sc1, sc1, sc1, sc2, sc1, sc2},
			pass: false,
		},
		{
			desc: "2 elements RR not RR, mistake in second iter",
			want: []apis.SubConn{sc1, sc2},
			got:  []apis.SubConn{sc1, sc2, sc1, sc1, sc1, sc2},
			pass: false,
		},
		{
			desc: "2 elements weighted RR",
			want: []apis.SubConn{sc1, sc1, sc2},
			got:  []apis.SubConn{sc1, sc1, sc2, sc1, sc1, sc2},
			pass: true,
		},
		{
			desc: "2 elements weighted RR different order",
			want: []apis.SubConn{sc1, sc1, sc2},
			got:  []apis.SubConn{sc1, sc2, sc1, sc1, sc2, sc1},
			pass: true,
		},

		{
			desc: "3 elements RR",
			want: []apis.SubConn{sc1, sc2, sc3},
			got:  []apis.SubConn{sc1, sc2, sc3, sc1, sc2, sc3, sc1, sc2, sc3},
			pass: true,
		},
		{
			desc: "3 elements RR different order",
			want: []apis.SubConn{sc1, sc2, sc3},
			got:  []apis.SubConn{sc3, sc2, sc1, sc3, sc2, sc1},
			pass: true,
		},
		{
			desc: "3 elements weighted RR",
			want: []apis.SubConn{sc1, sc1, sc1, sc2, sc2, sc3},
			got:  []apis.SubConn{sc1, sc2, sc3, sc1, sc2, sc1, sc1, sc2, sc3, sc1, sc2, sc1},
			pass: true,
		},
		{
			desc: "3 elements weighted RR not RR, mistake in first iter",
			want: []apis.SubConn{sc1, sc1, sc1, sc2, sc2, sc3},
			got:  []apis.SubConn{sc1, sc2, sc1, sc1, sc2, sc1, sc1, sc2, sc3, sc1, sc2, sc1},
			pass: false,
		},
		{
			desc: "3 elements weighted RR not RR, mistake in second iter",
			want: []apis.SubConn{sc1, sc1, sc1, sc2, sc2, sc3},
			got:  []apis.SubConn{sc1, sc2, sc3, sc1, sc2, sc1, sc1, sc1, sc3, sc1, sc2, sc1},
			pass: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			err := IsRoundRobin(tC.want, (&testClosure{r: tC.got}).next)
			if err == nil != tC.pass {
				t.Errorf("want pass %v, want %v, got err %v", tC.pass, tC.want, err)
			}
		})
	}
}
