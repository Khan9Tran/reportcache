package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Khan9Tran/reportcache"
	"github.com/Khan9Tran/reportcache/adapters/memory"
)

// SmartReportPolicy phân chia khoảng thời gian thành 3 vùng rõ rệt:
// 1. Quá khứ (Interval.To <= CurrentTime): Đã chốt sổ -> Cache dài hạn.
// 2. Hiện tại & Tương lai (Interval.To > CurrentTime): Dữ liệu chưa đóng sổ (Live / Future Booking)
//    -> Phải áp dụng TTL ngắn (LiveTTL) để cập nhật số liệu mới hoặc khách đặt thêm/hủy lịch.
type SmartReportPolicy struct {
	LiveTTL time.Duration
}

func (p *SmartReportPolicy) Evaluate(meta *reportcache.EntryMetadata, ctx reportcache.ValidityContext) reportcache.ValidityState {
	if meta == nil {
		return reportcache.StateMissing
	}
	if !meta.Completed {
		return reportcache.StateInvalidated
	}

	// 1. Nếu bucket đã kết thúc hoàn toàn trong quá khứ (ví dụ: ngày hôm qua)
	// Bucket.To <= CurrentTime -> Đã chốt sổ an toàn, giữ cache hợp lệ!
	if !meta.Identity.Interval.To.After(ctx.CurrentTime) {
		return reportcache.StateValid
	}

	// 2. Nếu bucket là Ngày Hôm Nay (đang diễn ra) hoặc Ngày Mai (tương lai chưa chốt)
	// Dữ liệu này vẫn đang biến động (đơn mới vào trong ngày, hoặc khách tiếp tục đặt lịch cho ngày mai)
	// -> Phải kiểm tra LiveTTL ngắn!
	if ctx.CurrentTime.Sub(meta.CreatedAt) > p.LiveTTL {
		return reportcache.StateStale // Hết hạn -> Cần refresh lại từ DB
	}

	return reportcache.StateValid
}

type OrderMetric struct {
	Date   string
	Orders int
	Note   string
}

func main() {
	ctx := context.Background()
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)

	// Giả lập thời điểm hiện tại: 25/11/2026 lúc 14:00 chiều
	mockNow := time.Date(2026, 11, 25, 14, 0, 0, 0, loc)
	clock := func() time.Time { return mockNow }

	cacheStore := memory.New[OrderMetric](clock)
	dayBucket, _ := reportcache.NewCalendarDayBucket(loc)

	// Khoảng thời gian: Hôm qua (24/11) đến hết Ngày mai (26/11)
	from := time.Date(2026, 11, 24, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 27, 0, 0, 0, 0, loc)
	iv, _ := reportcache.NewInterval(from, to)

	todayOrders := 45
	tomorrowBookings := 10

	fetchFromDB := func(ctx context.Context, id reportcache.Identity) (OrderMetric, error) {
		dateStr := id.Interval.From.Format("2006-01-02")
		switch id.Interval.From.Day() {
		case 24:
			return OrderMetric{Date: dateStr, Orders: 120, Note: "Đã chốt sổ (Past - Immutable)"}, nil
		case 25:
			return OrderMetric{Date: dateStr, Orders: todayOrders, Note: "Đang diễn ra (Today - Live)"}, nil
		case 26:
			return OrderMetric{Date: dateStr, Orders: tomorrowBookings, Note: "Đặt trước (Tomorrow - Future Booking)"}, nil
		default:
			return OrderMetric{Date: dateStr, Orders: 0}, nil
		}
	}

	req := reportcache.Request{
		Namespace:    "company-xyz",
		ReportID:     "order-metrics",
		ScopeKey:     "all-branches",
		QueryVersion: "v1",
		Interval:     iv,
		Bucket:       dayBucket,
	}

	// Cấu hình policy: Vùng dữ liệu chưa đóng sổ (Hôm nay & Ngày mai) chỉ cache tối đa 30 giây!
	smartPolicy := &SmartReportPolicy{LiveTTL: 30 * time.Second}

	// ==========================================
	// LẦN 1: Lúc 14:00 chiều (Cache trống)
	// ==========================================
	fmt.Println("=== [LẦN 1: 14:00] Truy vấn 3 ngày (Hôm qua - Hôm nay - Ngày mai) ===")
	res1, _ := reportcache.Resolve(ctx, req, cacheStore, fetchFromDB,
		reportcache.WithValidityPolicy(smartPolicy),
		reportcache.WithClock(clock),
	)
	printResults(res1)

	// ==========================================
	// LẦN 2: Lúc 14:00:15 (Sau 15 giây) -> Chưa hết TTL 30s
	// ==========================================
	mockNow = mockNow.Add(15 * time.Second)
	fmt.Println("\n=== [LẦN 2: 14:00:15 (Sau 15s)] Cả 3 ngày đều còn trong hạn Cache Hit ===")
	res2, _ := reportcache.Resolve(ctx, req, cacheStore, fetchFromDB,
		reportcache.WithValidityPolicy(smartPolicy),
		reportcache.WithClock(clock),
	)
	printResults(res2)

	// ==========================================
	// LẦN 3: Lúc 14:01 (Sau 1 phút) -> Đã hết TTL 30s!
	// - Hôm nay: có thêm 15 đơn phát sinh (45 -> 60)
	// - Ngày mai: có thêm 5 khách đặt bàn mới (10 -> 15)
	// ==========================================
	mockNow = mockNow.Add(45 * time.Second)
	todayOrders = 60         // Số đơn hôm nay tăng
	tomorrowBookings = 15   // Khách vừa đặt thêm 5 đơn cho ngày mai!

	fmt.Println("\n=== [LẦN 3: 14:01:00 (Sau 1 phút)] Hết TTL 30s của vùng chưa chốt sổ! ===")
	fmt.Println("   -> Hôm qua: VẪN CACHE HIT (vì đã đóng sổ)!")
	fmt.Println("   -> Hôm nay & Ngày mai: ĐƯỢC REFRESH và cập nhật số liệu mới nhất!")
	res3, _ := reportcache.Resolve(ctx, req, cacheStore, fetchFromDB,
		reportcache.WithValidityPolicy(smartPolicy),
		reportcache.WithClock(clock),
	)
	printResults(res3)
}

func printResults(res *reportcache.Result[OrderMetric]) {
	for _, b := range res.Buckets {
		source := "[FETCH FROM DB]"
		if b.FromCache {
			source = "[CACHE HIT     ]"
		}
		fmt.Printf("   %s Ngày %s: %3d đơn (%s)\n",
			source, b.Payload.Date, b.Payload.Orders, b.Payload.Note)
	}
}
