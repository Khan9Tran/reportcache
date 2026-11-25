package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Khan9/reportcache"
	"github.com/Khan9/reportcache/adapters/memory"
	"github.com/Khan9/reportcache/canonical"
)

// DailyRevenue là cấu trúc dữ liệu báo cáo do ứng dụng định nghĩa.
type DailyRevenue struct {
	Date         string
	OrderCount   int
	TotalRevenue int64
}

// SummaryReport là báo cáo tổng hợp cuối cùng sau khi ứng dụng merge các bucket.
type SummaryReport struct {
	TotalOrders  int
	TotalRevenue int64
	DailyDetails []DailyRevenue
}

func main() {
	ctx := context.Background()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	}

	// 1. Khởi tạo Cache Store (in-memory, hoặc sau này có thể cắm redis adapter)
	cacheStore := memory.New[DailyRevenue](nil)

	// 2. Định nghĩa bucket theo ngày dương lịch tại Việt Nam (Asia/Ho_Chi_Minh)
	dayBucket, err := reportcache.NewCalendarDayBucket(loc)
	if err != nil {
		panic(err)
	}

	// 3. Giả lập cơ sở dữ liệu và hàm fetch
	var dbQueryCount int64
	fetchFromDB := func(ctx context.Context, id reportcache.Identity) (DailyRevenue, error) {
		atomic.AddInt64(&dbQueryCount, 1)
		day := id.Interval.From.Day()
		// Giả lập số đơn và doanh thu theo ngày
		return DailyRevenue{
			Date:         id.Interval.From.Format("2006-01-02"),
			OrderCount:   day * 10,
			TotalRevenue: int64(day * 1000000),
		}, nil
	}

	// 4. Chuẩn hóa filter chi nhánh và loại tiền tệ thành ScopeKey
	rawOffices := []string{"office-002", "office-001", "office-002"} // Danh sách có thể bị lộn xộn/trùng lặp
	if err := canonical.ValidateNonEmptySlice(rawOffices); err != nil {
		panic(err)
	}
	canonicalOffices := canonical.Set(rawOffices) // -> "office-001,office-002"
	scopeKey := canonical.FormatKV(map[string]string{
		"currency": "VND",
		"offices":  canonicalOffices,
	})
	fmt.Printf("=== [BƯỚC 1] Chuẩn hóa ScopeKey: %s ===\n\n", scopeKey)

	// --- YÊU CẦU 1: Lấy báo cáo từ 01/11 đến 06/11 (5 ngày) ---
	from1 := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	to1 := time.Date(2026, 11, 6, 0, 0, 0, 0, loc)
	iv1, _ := reportcache.NewInterval(from1, to1)

	fmt.Println(">> [REQUEST 1] Lấy dữ liệu từ 01/11 đến 06/11 (Lần đầu, cache trống)")
	res1, err := reportcache.Resolve(ctx, reportcache.Request{
		Namespace:    "company-abc",
		ReportID:     "daily-revenue",
		ScopeKey:     scopeKey,
		QueryVersion: "v1",
		Interval:     iv1,
		Bucket:       dayBucket,
	}, cacheStore, fetchFromDB)
	if err != nil {
		panic(err)
	}

	printResults(res1)
	fmt.Printf("=> Tổng số lần query vào Database: %d ngày\n\n", atomic.LoadInt64(&dbQueryCount))

	// --- YÊU CẦU 2: Lấy dải ngày mở rộng từ 04/11 đến 09/11 (5 ngày: ngày 04, 05 đã có cache) ---
	from2 := time.Date(2026, 11, 4, 0, 0, 0, 0, loc)
	to2 := time.Date(2026, 11, 9, 0, 0, 0, 0, loc)
	iv2, _ := reportcache.NewInterval(from2, to2)

	fmt.Println(">> [REQUEST 2] Lấy dải ngày gối đầu từ 04/11 đến 09/11")
	fmt.Println("   (Kỳ vọng: Ngày 04 và 05 tái sử dụng cache; Chỉ fetch DB ngày 06, 07, 08)")
	res2, err := reportcache.Resolve(ctx, reportcache.Request{
		Namespace:    "company-abc",
		ReportID:     "daily-revenue",
		ScopeKey:     scopeKey,
		QueryVersion: "v1",
		Interval:     iv2,
		Bucket:       dayBucket,
	}, cacheStore, fetchFromDB)
	if err != nil {
		panic(err)
	}

	printResults(res2)
	fmt.Printf("=> Tổng số lần query vào Database sau Request 2: %d ngày (+3 ngày mới thay vì 5)\n\n", atomic.LoadInt64(&dbQueryCount))

	// --- YÊU CẦU 3: Phát sinh sự kiện sửa đổi dữ liệu nguồn -> Tăng Generation ---
	fmt.Println(">> [REQUEST 3] Phát sinh sự kiện sửa đổi dữ liệu nguồn!")
	fmt.Println("   Nâng Report Generation lên 2 (Strategy B). Yêu cầu lại cùng dải ngày 04/11 đến 09/11.")
	res3, err := reportcache.Resolve(ctx, reportcache.Request{
		Namespace:    "company-abc",
		ReportID:     "daily-revenue",
		ScopeKey:     scopeKey,
		QueryVersion: "v1",
		Interval:     iv2,
		Bucket:       dayBucket,
	}, cacheStore, fetchFromDB,
		reportcache.WithGeneration(2),       // Ghi cache với thế hệ 2
		reportcache.WithReportGeneration(2), // Chỉ chấp nhận cache thế hệ >= 2
	)
	if err != nil {
		panic(err)
	}

	printResults(res3)
	fmt.Printf("=> Toàn bộ 5 ngày đều được fetch mới an toàn! Tổng DB query: %d ngày (+5)\n\n", atomic.LoadInt64(&dbQueryCount))

	// 5. Ứng dụng tự gộp dữ liệu theo logic nghiệp vụ của mình
	summary := mergeDailyRevenue(res3.Buckets)
	fmt.Println("=== [KẾT QUẢ MERGE NGHIỆP VỤ CỦA ỨNG DỤNG] ===")
	fmt.Printf("Tổng số đơn hàng : %d đơn\n", summary.TotalOrders)
	fmt.Printf("Tổng doanh thu   : %s VND\n", formatMoney(summary.TotalRevenue))
}

func printResults(res *reportcache.Result[DailyRevenue]) {
	for _, b := range res.Buckets {
		source := "[FETCH FROM DB]"
		if b.FromCache {
			source = "[CACHE HIT     ]"
		}
		fmt.Printf("   %s Ngày %s: %3d đơn | %10s VND\n",
			source, b.Payload.Date, b.Payload.OrderCount, formatMoney(b.Payload.TotalRevenue))
	}
}

func mergeDailyRevenue(buckets []reportcache.BucketResult[DailyRevenue]) SummaryReport {
	var summary SummaryReport
	for _, b := range buckets {
		summary.TotalOrders += b.Payload.OrderCount
		summary.TotalRevenue += b.Payload.TotalRevenue
		summary.DailyDetails = append(summary.DailyDetails, b.Payload)
	}
	return summary
}

func formatMoney(n int64) string {
	in := fmt.Sprintf("%d", n)
	out := ""
	for i, c := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			out += "."
		}
		out += string(c)
	}
	return out
}
