package datatypes

type Order struct {
	Id         string `json:"id"` // customerid_bktseqno
	OrderDate  string `json:"orderDate"`
	CustomerId string `json:"customerId"`
}

func (rec Order) RecId() string {
	return rec.Id
}

type OrderItem struct {
	Id        string `json:"id"` // orderid_itemno
	OrderId   string `json:"orderId"`
	ItemNo    int    `json:"itemNo"`
	ProductId string `json:"productId"`
	Qty       int    `json:"qty"`
}

func (rec OrderItem) RecId() string {
	return rec.Id
}
