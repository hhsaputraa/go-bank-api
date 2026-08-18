package logger

import (
	"testing"
	"time"
)

func BenchmarkRecordActivityConcurrent(b *testing.B) {
	InitLogger()
	defer CloseLogger()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			RecordActivity(
				"192.168.1.100",
				"tampilkan total saldo",
				"SQL",
				"SELECT SUM(saldo) FROM tabungan",
				"SUCCESS",
				nil,
				12*time.Millisecond,
			)
		}
	})
}
