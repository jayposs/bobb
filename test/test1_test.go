package test

import (
	"fmt"
	"log"
	"net/http"

	"reflect"
	"slices"
	"testing"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"
)

const (
	orderBkt     = "test1_order"
	orderItemBkt = "test1_order_item"
	indexBkt     = orderBkt + "_orderdate_index"
)

func Test1(t *testing.T) {

	log.Println("-- Test1 starting -----")

	bo.BaseURL = "http://localhost:50555/"
	bo.Debug = true

	httpClient := &http.Client{}

	bo.DeleteBkt(httpClient, orderBkt)
	bo.DeleteBkt(httpClient, orderItemBkt)
	bo.DeleteBkt(httpClient, indexBkt)

	if err := loadIndexSettings(httpClient); err != nil {
		t.Error(err)
	}
	if err := loadData(httpClient); err != nil {
		t.Error(err)
	}
	//getData()
	//qryData()
}

func loadIndexSettings(httpClient *http.Client) error {
	log.Println("-- test1 loadIndexSettings starting -----")

	// create index settings for order bucket
	orderDateIndex := bobb.IndexSetting{
		DataBkt:        orderBkt,
		IndexBkt:       indexBkt,
		KeySuffixWidth: 6, // to make unique, add index key suffix from orderbkt nextseq, 6 digits with leading zeros
		KeyFlds: []bobb.FldFormat{
			{FldName: "orderDate", FldType: bobb.FldTypeStr, Length: 10, StrOption: bobb.StrAsIs, UseDefault: bobb.DefaultNever},
		},
		FldSeparator: "|",
		SkipOnErr:    false,
	}
	resp, err := bo.Run(httpClient, bobb.OpIndexSetting, bobb.IndexSettingRequest{
		IndexSettings: []bobb.IndexSetting{
			orderDateIndex,
		},
	})
	if err := checkResp(resp, err, "loadIndexSettings"); err != nil {
		return fmt.Errorf("loadIndexSettings failed: %v", err)
	}

	// verify index setting in db matches sent index setting

	indexKey := orderDateIndex.IndexBkt
	var savedIndexSetting bobb.IndexSetting

	err = bo.GetOne(httpClient, "index_settings", indexKey, &savedIndexSetting)
	if err != nil {
		return fmt.Errorf("loadIndexSettings getone failed: %v", err)
	}
	if reflect.DeepEqual(orderDateIndex, savedIndexSetting) == false {
		return fmt.Errorf("loadIndexSettings db index setting does not match sent index setting")
	}

	log.Println("-- test1 loadIndexSettings done -----")
	return nil
}

func loadData(httpClient *http.Client) error {
	log.Println("-- test1 loadData starting -----")

	orderSeqNos, _ := bo.GetSeqNos(httpClient, orderBkt, 3, 5) // get unique sequence numbers for order key suffix
	if slices.Compare(orderSeqNos, []string{"00001", "00002", "00003"}) != 0 {
		return fmt.Errorf("orderSeqNos do not match expected values: %v", orderSeqNos)
	}

	var orders = []Order{
		{Id: "custA_" + orderSeqNos[0], OrderDate: "2024-05-23", CustomerId: "custA"},
		{Id: "custB_" + orderSeqNos[1], OrderDate: "2024-06-24", CustomerId: "custB"},
		{Id: "custC_" + orderSeqNos[2], OrderDate: "2024-07-25", CustomerId: "custC"},
	}
	var expectedIndexEntries = map[string]string{ // orderdate|keysuffix : orderid
		"2024-05-23|000001": "custA_00001",
		"2024-06-24|000002": "custB_00002",
		"2024-07-25|000003": "custC_00003",
	}
	var items = []OrderItem{
		{Id: orders[0].Id + "_1", OrderId: orders[0].Id, ItemNo: 1, ProductId: "prodid_a", Qty: 2},
		{Id: orders[0].Id + "_2", OrderId: orders[0].Id, ItemNo: 2, ProductId: "prodid_a", Qty: 1},
		{Id: orders[0].Id + "_3", OrderId: orders[0].Id, ItemNo: 3, ProductId: "prodid_x", Qty: 5},
		{Id: orders[1].Id + "_1", OrderId: orders[1].Id, ItemNo: 1, ProductId: "prodid_b", Qty: 3},
		{Id: orders[2].Id + "_1", OrderId: orders[2].Id, ItemNo: 1, ProductId: "prodid_c", Qty: 4},
	}

	jsonOrders := bo.SliceToJson(orders)
	jsonItems := bo.SliceToJson(items)

	putParm1 := bobb.PutParm{
		BktName: orderBkt,
		Recs:    jsonOrders,
	}
	putParm2 := bobb.PutParm{
		BktName: orderItemBkt,
		Recs:    jsonItems,
	}

	resp, err := bo.Run(httpClient, bobb.OpPut, bobb.PutRequest{
		PutParms: []bobb.PutParm{putParm1, putParm2},
	})
	if err := checkResp(resp, err, "loadData"); err != nil {
		return fmt.Errorf("loadData failed: %v", err)
	}
	// verify orders in db matches order sent
	for _, order := range orders {
		var savedOrder Order
		bo.GetOne(httpClient, orderBkt, order.Id, &savedOrder)
		if order != savedOrder {
			return fmt.Errorf("loadData db order does not match sent order for order %s", order.Id)
		}
	}
	// verify order items in db matches items sent
	for _, item := range items {
		var savedItem OrderItem
		bo.GetOne(httpClient, orderItemBkt, item.Id, &savedItem)
		if item != savedItem {
			return fmt.Errorf("loadData db order item does not match sent order item for item %s", item.Id)
		}
	}
	// display index keys
	resp, _ = bo.Run(httpClient, bobb.OpGetAllKeys, bobb.GetAllKeysRequest{BktName: indexBkt})
	for _, key := range resp.Recs {
		log.Printf("index key: %s", string(key))
	}
	// display index values
	resp, _ = bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{BktName: indexBkt})
	for _, val := range resp.Recs {
		log.Printf("index value: %s", string(val))
	}
	// verify index entries for orderDate index match expected values
	for indexKey, orderId := range expectedIndexEntries {
		resp, _ := bo.Run(httpClient, bobb.OpGetOne, bobb.GetOneRequest{BktName: indexBkt, Key: indexKey})
		if string(resp.Rec) != orderId {
			return fmt.Errorf("loadData db index value does not match expected value for key %s: expected %s, got %s", indexKey, orderId, string(resp.Rec))
		}
	}
	log.Println("-- test1 loadData done -----")
	return nil
}

func checkResp(resp *bobb.Response, err error, desc string) error {
	if resp == nil {
		return fmt.Errorf("nil response, check display for details")
	}
	if !(resp.Status == bobb.StatusOk && err == nil) {
		return fmt.Errorf("%s failed: %s, error: %v", desc, resp.Msg, err)
	}
	return nil
}
