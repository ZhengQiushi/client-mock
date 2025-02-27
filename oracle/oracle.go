// Copyright 2021 TiKV Authors
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

// NOTE: The code in this file is based on code from the
// TiDB project, licensed under the Apache License v 2.0
//
// https://github.com/pingcap/tidb/tree/cc5e161ac06827589c4966674597c137cc9e809c/store/tikv/oracle/oracle.go
//

// Copyright 2016 PingCAP, Inc.
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

package oracle

import (
	"context"
	"time"
)

// Option represents available options for the oracle.Oracle.
type Option struct {
	TxnScope string
}

// Oracle is the interface that provides strictly ascending timestamps.
type Oracle interface {
	GetTimestamp(ctx context.Context, opt *Option) (uint64, error)
	GetTimestampAsync(ctx context.Context, opt *Option) Future
	GetLowResolutionTimestamp(ctx context.Context, opt *Option) (uint64, error)
	GetLowResolutionTimestampAsync(ctx context.Context, opt *Option) Future
	SetLowResolutionTimestampUpdateInterval(time.Duration) error
	// GetStaleTimestamp generates a timestamp based on the recently fetched timestamp and the elapsed time since
	// when that timestamp was fetched. The result is expected to be about `prevSecond` seconds before the current
	// time.
	// WARNING: This method does not guarantee whether the generated timestamp is legal for accessing the data.
	// Neither is it safe to use it for verifying the legality of another calculated timestamp.
	// Be sure to validate the timestamp before using it to access the data.
	GetStaleTimestamp(ctx context.Context, txnScope string, prevSecond uint64) (uint64, error)
	IsExpired(lockTimestamp, TTL uint64, opt *Option) bool
	UntilExpired(lockTimeStamp, TTL uint64, opt *Option) int64
	Close()

	GetExternalTimestamp(ctx context.Context) (uint64, error)
	SetExternalTimestamp(ctx context.Context, ts uint64) error

	// GetAllTSOKeyspaceGroupMinTS gets a minimum timestamp from all TSO keyspace groups.
	GetAllTSOKeyspaceGroupMinTS(ctx context.Context) (uint64, error)

	// ValidateSnapshotReadTS verifies whether it can be guaranteed that the given readTS doesn't exceed the maximum ts
	// that has been allocated by the oracle, so that it's safe to use this ts to perform snapshot read, stale read,
	// etc.
	// Note that this method only checks the ts from the oracle's perspective. It doesn't check whether the snapshot
	// has been GCed.
	ValidateSnapshotReadTS(ctx context.Context, readTS uint64, opt *Option) error
}

// Future is a future which promises to return a timestamp.
type Future interface {
	Wait() (uint64, error)
}

const (
	physicalShiftBits = 18
	logicalBits       = (1 << physicalShiftBits) - 1
	// GlobalTxnScope is the default transaction scope for an Oracle service.
	GlobalTxnScope = "global"
)

// ComposeTS creates a ts from physical and logical parts.
func ComposeTS(physical, logical int64) uint64 {
	return uint64((physical << physicalShiftBits) + logical)
}

// ExtractPhysical returns a ts's physical part.
func ExtractPhysical(ts uint64) int64 {
	return int64(ts >> physicalShiftBits)
}

// ExtractLogical return a ts's logical part.
func ExtractLogical(ts uint64) int64 {
	return int64(ts & logicalBits)
}

// GetPhysical returns physical from an instant time with millisecond precision.
func GetPhysical(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

// GetTimeFromTS extracts time.Time from a timestamp.
func GetTimeFromTS(ts uint64) time.Time {
	ms := ExtractPhysical(ts)
	return time.Unix(ms/1e3, (ms%1e3)*1e6)
}

// GoTimeToTS converts a Go time to uint64 timestamp.
func GoTimeToTS(t time.Time) uint64 {
	ts := (t.UnixNano() / int64(time.Millisecond)) << physicalShiftBits
	return uint64(ts)
}

// GoTimeToLowerLimitStartTS returns the min start_ts of the uncommitted transaction.
// maxTxnTimeUse means the max time a Txn May use (in ms) from its begin to commit.
func GoTimeToLowerLimitStartTS(now time.Time, maxTxnTimeUse int64) uint64 {
	return GoTimeToTS(now.Add(-time.Duration(maxTxnTimeUse) * time.Millisecond))
}
