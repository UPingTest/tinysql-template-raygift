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
// See the License for the specific language governing permissions and
// limitations under the License.

package tablecodec

import (
	"fmt"
	"math"
	"testing"
	"time"

	. "github.com/pingcap/check"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/parser/mysql"
	"github.com/pingcap/tidb/parser/terror"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/testleak"
)

func TestT(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&testTableCodecSuite{})

type testTableCodecSuite struct{}

// TestTableCodec  tests some functions in package tablecodec
// TODO: add more tests.
func (s *testTableCodecSuite) TestTableCodec(c *C) {
	defer testleak.AfterTest(c)()
	key := EncodeRowKey(1, codec.EncodeInt(nil, 2))
	h, err := DecodeRowKey(key)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, int64(2))

	key = EncodeRowKeyWithHandle(1, 2)
	// c.Errorf("Type: %T Value: %v\n", key, key)
	h, err = DecodeRowKey(key)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, int64(2))
}

func (s *testTableCodecSuite) TestCutKeyNew(c *C) {
	values := []types.Datum{types.NewIntDatum(1), types.NewBytesDatum([]byte("abc")), types.NewFloat64Datum(5.5)}
	handle := types.NewIntDatum(100)
	values = append(values, handle)
	sc := &stmtctx.StatementContext{TimeZone: time.UTC}
	encodedValue, err := codec.EncodeKey(sc, nil, values...)
	c.Assert(err, IsNil)
	tableID := int64(4)
	indexID := int64(5)
	indexKey := EncodeIndexSeekKey(tableID, indexID, encodedValue)
	valuesBytes, handleBytes, err := CutIndexKeyNew(indexKey, 3)
	c.Assert(err, IsNil)
	for i := 0; i < 3; i++ {
		valueBytes := valuesBytes[i]
		var val types.Datum
		_, val, _ = codec.DecodeOne(valueBytes)
		c.Assert(val, DeepEquals, values[i])
	}
	_, handleVal, _ := codec.DecodeOne(handleBytes)
	c.Assert(handleVal, DeepEquals, types.NewIntDatum(100))
}

func (s *testTableCodecSuite) TestCutKey(c *C) {
	colIDs := []int64{1, 2, 3}
	values := []types.Datum{types.NewIntDatum(1), types.NewBytesDatum([]byte("abc")), types.NewFloat64Datum(5.5)}
	handle := types.NewIntDatum(100)
	values = append(values, handle)
	sc := &stmtctx.StatementContext{TimeZone: time.UTC}
	encodedValue, err := codec.EncodeKey(sc, nil, values...)
	c.Assert(err, IsNil)
	tableID := int64(4)
	indexID := int64(5)
	indexKey := EncodeIndexSeekKey(tableID, indexID, encodedValue)
	valuesMap, handleBytes, err := CutIndexKey(indexKey, colIDs)
	c.Assert(err, IsNil)
	for i, colID := range colIDs {
		valueBytes := valuesMap[colID]
		var val types.Datum
		_, val, _ = codec.DecodeOne(valueBytes)
		c.Assert(val, DeepEquals, values[i])
	}
	_, handleVal, _ := codec.DecodeOne(handleBytes)
	c.Assert(handleVal, DeepEquals, types.NewIntDatum(100))
}

func (s *testTableCodecSuite) TestIndexKey(c *C) {
	tableID := int64(4)
	indexID := int64(5)
	indexKey := EncodeIndexSeekKey(tableID, indexID, []byte{})
	c.Error(indexKey)
	tTableID, tIndexID, isRecordKey, err := DecodeKeyHead(indexKey)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)
	c.Assert(tIndexID, Equals, indexID)
	c.Assert(isRecordKey, IsFalse)
}

func (s *testTableCodecSuite) TestRecordKey(c *C) {
	tableID := int64(55)
	tableKey := EncodeRowKeyWithHandle(tableID, math.MaxUint32)
	tTableID, _, isRecordKey, err := DecodeKeyHead(tableKey)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)
	c.Assert(isRecordKey, IsTrue)

	encodedHandle := codec.EncodeInt(nil, math.MaxUint32)
	rowKey := EncodeRowKey(tableID, encodedHandle)
	c.Assert([]byte(tableKey), BytesEquals, []byte(rowKey))
	tTableID, handle, err := DecodeRecordKey(rowKey)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)
	c.Assert(handle, Equals, int64(math.MaxUint32))

	recordPrefix := GenTableRecordPrefix(tableID)
	rowKey = EncodeRecordKey(recordPrefix, math.MaxUint32)
	c.Assert([]byte(tableKey), BytesEquals, []byte(rowKey))

	_, _, err = DecodeRecordKey(nil)
	c.Assert(err, NotNil)
	_, _, err = DecodeRecordKey([]byte("abcdefghijklmnopqrstuvwxyz"))
	c.Assert(err, NotNil)
	c.Assert(DecodeTableID(nil), Equals, int64(0))
}

func (s *testTableCodecSuite) TestPrefix(c *C) {
	const tableID int64 = 66
	key := EncodeTablePrefix(tableID)
	tTableID := DecodeTableID(key)
	c.Assert(tTableID, Equals, int64(tableID))

	c.Assert([]byte(TablePrefix()), BytesEquals, tablePrefix)

	tablePrefix1 := GenTablePrefix(tableID)
	c.Assert([]byte(tablePrefix1), BytesEquals, []byte(key))

	indexPrefix := EncodeTableIndexPrefix(tableID, math.MaxUint32)
	tTableID, indexID, isRecordKey, err := DecodeKeyHead(indexPrefix)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)
	c.Assert(indexID, Equals, int64(math.MaxUint32))
	c.Assert(isRecordKey, IsFalse)

	prefixKey := GenTableIndexPrefix(tableID)
	c.Assert(DecodeTableID(prefixKey), Equals, tableID)

	c.Assert(TruncateToRowKeyLen(append(indexPrefix, "xyz"...)), HasLen, RecordRowKeyLen)
	c.Assert(TruncateToRowKeyLen(key), HasLen, len(key))
}

func (s *testTableCodecSuite) TestReplaceRecordKeyTableID(c *C) {
	tableID := int64(1)
	tableKey := EncodeRowKeyWithHandle(tableID, 1)
	tTableID, _, _, err := DecodeKeyHead(tableKey)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)

	tableID = 2
	tableKey = ReplaceRecordKeyTableID(tableKey, tableID)
	tTableID, _, _, err = DecodeKeyHead(tableKey)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)

	tableID = 3
	ReplaceRecordKeyTableID(tableKey, tableID)
	tableKey = ReplaceRecordKeyTableID(tableKey, tableID)
	tTableID, _, _, err = DecodeKeyHead(tableKey)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)

	tableID = -1
	tableKey = ReplaceRecordKeyTableID(tableKey, tableID)
	tTableID, _, _, err = DecodeKeyHead(tableKey)
	c.Assert(err, IsNil)
	c.Assert(tTableID, Equals, tableID)
}

func (s *testTableCodecSuite) TestDecodeIndexKey(c *C) {
	tableID := int64(4)
	indexID := int64(5)
	values := []types.Datum{
		types.NewIntDatum(1),
		types.NewBytesDatum([]byte("abc")),
		types.NewFloat64Datum(123.45),
		// MysqlTime is not supported.
		// types.NewTimeDatum(types.Time{
		// 	Time: types.FromGoTime(time.Now()),
		// 	Fsp:  6,
		// 	Type: mysql.TypeTimestamp,
		// }),
	}
	valueStrs := make([]string, 0, len(values))
	for _, v := range values {
		str, err := v.ToString()
		if err != nil {
			str = fmt.Sprintf("%d-%v", v.Kind(), v.GetValue())
		}
		valueStrs = append(valueStrs, str)
	}
	sc := &stmtctx.StatementContext{TimeZone: time.UTC}
	encodedValue, err := codec.EncodeKey(sc, nil, values...)
	c.Assert(err, IsNil)
	indexKey := EncodeIndexSeekKey(tableID, indexID, encodedValue)

	decodeTableID, decodeIndexID, decodeValues, err := DecodeIndexKey(indexKey)
	c.Assert(err, IsNil)
	c.Assert(decodeTableID, Equals, tableID)
	c.Assert(decodeIndexID, Equals, indexID)
	c.Assert(decodeValues, DeepEquals, valueStrs)
}

func (s *testTableCodecSuite) TestCutPrefix(c *C) {
	key := EncodeTableIndexPrefix(42, 666)
	res := CutRowKeyPrefix(key)
	c.Assert(res, BytesEquals, []byte{0x80, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2, 0x9a})
	res = CutIndexPrefix(key)
	c.Assert(res, BytesEquals, []byte{})
}

func (s *testTableCodecSuite) TestRange(c *C) {
	s1, e1 := GetTableHandleKeyRange(22)
	s2, e2 := GetTableHandleKeyRange(23)
	c.Assert([]byte(s1), Less, []byte(e1))
	c.Assert([]byte(e1), Less, []byte(s2))
	c.Assert([]byte(s2), Less, []byte(e2))

	s1, e1 = GetTableIndexKeyRange(42, 666)
	s2, e2 = GetTableIndexKeyRange(42, 667)
	c.Assert([]byte(s1), Less, []byte(e1))
	c.Assert([]byte(e1), Less, []byte(s2))
	c.Assert([]byte(s2), Less, []byte(e2))
}

func (s *testTableCodecSuite) TestDecodeAutoIDMeta(c *C) {
	keyBytes := []byte{0x6d, 0x44, 0x42, 0x3a, 0x35, 0x36, 0x0, 0x0, 0x0, 0xfc, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x68, 0x54, 0x49, 0x44, 0x3a, 0x31, 0x30, 0x38, 0x0, 0xfe}
	key, field, err := DecodeMetaKey(kv.Key(keyBytes))
	c.Assert(err, IsNil)
	c.Assert(string(key), Equals, "DB:56")
	c.Assert(string(field), Equals, "TID:108")
}

func BenchmarkHasTablePrefix(b *testing.B) {
	k := kv.Key("foobar")
	for i := 0; i < b.N; i++ {
		hasTablePrefix(k)
	}
}

func BenchmarkHasTablePrefixBuiltin(b *testing.B) {
	k := kv.Key("foobar")
	for i := 0; i < b.N; i++ {
		k.HasPrefix(tablePrefix)
	}
}

// Bench result:
// BenchmarkEncodeValue      5000000           368 ns/op
func BenchmarkEncodeValue(b *testing.B) {
	row := make([]types.Datum, 7)
	row[0] = types.NewIntDatum(100)
	row[1] = types.NewBytesDatum([]byte("abc"))
	row[2] = types.NewFloat32Datum(1.5)
	b.ResetTimer()
	encodedCol := make([]byte, 0, 16)
	for i := 0; i < b.N; i++ {
		for _, d := range row {
			encodedCol = encodedCol[:0]
			EncodeValue(nil, encodedCol, d)
		}
	}
}

func (s *testTableCodecSuite) TestError(c *C) {
	kvErrs := []*terror.Error{
		errInvalidKey,
		errInvalidRecordKey,
		errInvalidIndexKey,
	}
	for _, err := range kvErrs {
		code := err.ToSQLError().Code
		c.Assert(code != mysql.ErrUnknown && code == uint16(err.Code()), IsTrue, Commentf("err: %v", err))
	}
}
