## bobb - JSON Data Store With Server   

### Status
All features have been tested, but I have not personally used bobb for any projects.  

### Built on high performance key-value store [bolt/bbolt](https://github.com/etcd-io/bbolt) 

### Key Features
* simple http server allows multiple programs to access bolt database
* go http client pkg makes interfacing with server super easy
* query (filter, sort)
* secondary indexes

### Testing
Program demo/demo.go verifies all db request types work correctly using small test data set.  
Program bigqry/bigqry.go was used to test larger datasets (up to aprox 1 million recs).  
Program stress/stress.go was run for several hours using 80,000+ record dataset.  
Performance was consistent. Memory usage stayed constant.

### Why Bobb
* small, simple, fast
* more functionality than basic key-value store
* customizations are reasonably easy to make (see info/customize_howto.txt)

### Why Not Bobb
* lack of features
* lack of testing
* not robust enough
* not for large serious production apps

### Motivation
* database services have drawbacks such as cost and location  
* most popular databases are large, complex, and have tons of features I probably won't use
* installing, upgrading, and running them can be a little nerve racking 
* adding your own features is pretty much out of the question   

### Key Points
* bobb is a thin layer on top of bolt
* if you understand how bolt works, you will get bobb
* review code in view_handlers.go or updt_handlers.go to get a feel for how bobb works

### Guide To Documentation
* [long-readme.md](long-readme.md) - general discussion and explanations of project
* [info/api.md](info/api.md)- reference guide
* [info/a-starthere.txt](info/a-starthere.txt) - getting started steps
* [info/install.txt](info/install.txt) - setup instructions
* [info/ideas.txt](info/ideas.txt) - ideas for new features
* [info/changelog.txt](info/changelog.txt) - details of each project change
* [types.go](types.go) - detail documentation on each request type  

### Code Design
IMO the code is very direct and easy to follow.  
Server request url determines what handler func is called. Ex. url /putone > PutOne().   
The handler func performs actions and creates response.  
Read handlers using bolt View function are in view_handlers.go.  
Update handlers using bolt Update function are in updt_handlers.go.  
Support funcs that operate on individual records are in rec.go.  

### Make It Your Own
See info/customize_howto.txt for instructions on how to add features.   
I recommend making bobb your own project.  
Check info/changelog.txt periodically for new features/fixes you may want to add.  

## FYI - How Much Is Too Much
From SQLite web site, as of version 3.42.0 (2023-05-16), the SQLite library consists of approximately 155.8 KSLOC of C code. (KSLOC means thousands of "Source Lines Of Code" or, in other words, lines of code excluding blank lines and comments.) 

MySQL, PostgreSQL, MongoDB are much larger.

Checking mongodb on github (the underlying key-value engine, WiredTiger, is separate project):  
 -mongo/src/mongo/ - 24 sub folders  
 -mongo/src/mongo/db - aprox 400 files + 28 sub folders  
 -mongo/src/mongo/db/auth - aprox 180 files  
 -mongo/src/mongo/db/catalog - aprox 150 files  
 -mongo/src/mongo/db/exec - aprox 150 files + 4 sub folders  
 -mongo/src/mongo/db/pipeline - aprox 500 files + 6 sub folders  
 -mongo/src/mongo/db/query - aprox 250 files + 22 sub folders  
 -mongo/src/mongo/db/query/optimizer - 20 files + 4 sub folders  
 -mongo/src/mongo/db/query/optimizer/rewrites - 9 files  
