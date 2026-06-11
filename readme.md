## Bobb - JSON database built on [Bolt/Bbolt (etcd-io/bbolt)](https://github.com/etcd-io/bbolt)

<img src="wheelbarrow1.jpg" width="100" height="100">

Bobb attempts to find a good balance of small code size, simplicity, speed, and usefulness. It is a thin layer on top of the key-value data store, bolt/bbolt. 

### Documentation
* Folder "info" contains documentation files
* See [install.txt](info/install.txt) for setup steps
* See [guide-to-bobb.md](info/guide-to-bobb.md) for general information
    
### Key Features
* Http Server that allows multiple programs to simultaneously access the same database
* Client package that makes interacting with the server as easy as using an embedded db
* Secondary Indexes
* Queries supporting multiple search criteria with results returned in sorted order
* Simple Joins allowing values from related records to be included in results

### Example Request 
```
    import (
        ...
        "github.com/jayposs/bobb"
	    bo "github.com/jayposs/bobb/client"
    )
    ...
    req := bobb.GetAllRequest{
		BktName:  "inquiry",
		IndexBkt: "inquiry_timestamp_index",
		StartKey: "2021-01-00 00:00:00",
		EndKey:   "2021-03-31 99:99:99",
	}
	resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
	if resp.Status != bobb.StatusOK {
		log.Println(resp.Msg)
	}
	results := bo.JsonToMap(resp.Recs, Inquiry{}) // convert resp.Recs ([][]byte) to map of Inquiry recs

```

