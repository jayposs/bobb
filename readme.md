## Bobb - JSON database built on [Bolt/Bbolt (etcd-io/bbolt)](https://github.com/etcd-io/bbolt)

Bobb attempts to find a good balance of small code size, simplicity, speed, and usefulness. It is a thin layer on top of the key-value data store, Bolt. Understanding how Bolt works is important. For example, when using a key/index range to limit record input, requests can run at hyper speed. Bobb is easy to use, but places a lot of responsibility on the developer.  

### Documentation
* Folder "info" contains documentation files
* See info/a-index.md for description and link to each
    
### Features
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
    criteria := []bobb.FindCondition{
		{Fld: "zip", Op: bobb.FindStartsWith, ValStr: "54"},
        {Fld: "locationType", Op: bobb.FindEquals: ValInt: 3},
	} 
	sortKeys := []bobb.SortKey{
		{Fld: "address", Dir: bobb.SortAscStr},
	}
	req := bobb.QryRequest{
		BktName:        "location",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
    if resp.Status != bobb.StatusOk {
        log.Println(resp.Msg)
    }
    result := bo.JsonToSlice(resp.Recs, Location{})  // result is []Location recs  

```
A number of "shortcut" functions are included to reduce coding. For example, bo.GetOne(..) returns a single record into a target variable in a single line of code. 

### [JSON Schema](https://json-schema.org/) 

I have not used it, but looks like a good way to validate json data.  
Example Go pkg: https://github.com/kaptinlin/jsonschema.   

### Performance (example elapsed clock time between request sent, response received)
* Test system: 5yr old sff system76, Ubuntu 22.04, 25 watt 2 core/4 thread mobile processor, 8GB ram, ssd
* Primary data bucket: 166,700 records
* Qry with 1 find criteria, 2 sort parms, no index, 2546 result recs:  < .2 secs 
* Qry with 4 find criteria, 4 sort parms, no index, 1462 result recs:  < .2 secs 
* Get with no find criteria or sort parms, using index range, 70,600 result recs:  < .5 secs 
* Qry with 1 find critera, 1 sort parm, using index range, 4,536 result recs:  < .07 secs 
* Qry with 1 find critera, 1 sort parm, using index range, 58 result recs:  < .004 secs 
* Get with no find critera, no sort parms, using primary key prefix, 14 result recs:  < .001 secs
* Qry with 1 join, 1 find critera, 1 sort parm, using index range, 63,583 result recs:  < 1.02 secs  
* Batch load 166,700 records - 7 secs
* Load 1 index for 166,700 records - 5 secs

### Status (Dec 2025)
I am having some health issues and not sure what my level of effort will be. I think Bobb can be a useful project. The design is probably lacking, but I am amazed at how fast it runs. This quality is mainly due to Bolt, Fastjson, and Go. My hope is someone smarter than me will take Bobb to the next level and many programmers will put it in their toolbox.  
  
All features have been tested and confirmed using a very small dataset (see demo/demo.go). Volume tests run successfully, but results only randomly visually confirmed. Long running stress tests indicate memory use and performance remain constant.

## History  
Bobb did not start off as an intentional project. I began experimenting with some ideas just out of curiosity and over time, I felt like a real project was in sight. I don't consider myself to be knowledgeable enough to create a true database, but IMO Bobb is pretty awesome.  

## FYI - How Much Is Too Much  

Based on the following statistics, I would assume Bobb is completely unworthy of consideration, but I post it anyway.  

From SQLite web site:  
 As of version 3.42.0 (2023-05-16), the SQLite library consists of approximately 155.8 KSLOC of C code. (KSLOC means thousands of "Source Lines Of Code" or, in other words, lines of code excluding blank lines and comments.) 

From DuckDB 1.0.0 announcement:  
There are now over 300 000 lines of C++ engine code, over 42 000 commits and almost 4 000 issues were opened and closed again. 

I would assume MySQL, PostgreSQL, and MongoDB are much larger.

Checking MongoDB on Github (the underlying key-value engine, WiredTiger, is separate project):  
 -mongo/src/mongo/ - 24 sub folders  
 -mongo/src/mongo/db - aprox 400 files + 28 sub folders  
 -mongo/src/mongo/db/auth - aprox 180 files  
 -mongo/src/mongo/db/catalog - aprox 150 files  
 -mongo/src/mongo/db/exec - aprox 150 files + 4 sub folders  
 -mongo/src/mongo/db/pipeline - aprox 500 files + 6 sub folders  
 -mongo/src/mongo/db/query - aprox 250 files + 22 sub folders  
 -mongo/src/mongo/db/query/optimizer - 20 files + 4 sub folders  
 -mongo/src/mongo/db/query/optimizer/rewrites - 9 files  


