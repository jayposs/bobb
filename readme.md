## bobb - database module   

### Status
All features have been tested, but I have not personally used bobb for any projects.  

### Uses high performance key-value db bolt/bbolt 

### Key Features
* simple http server allows multiple programs to access bolt database
* query (filter, sort)
* secondary indexes

### Why Bobb
* small and simple
* more functionality than basic key-value database
* customizations are reasonably simple to make 

### Motivation
Using database services has drawbacks (cost, location). The big popular databases 
have more features than I needed and are complex. Installing, upgrading, and running can be a 
little nerve racking. Adding your own features is pretty much out of the question.   

### Key Points
1. bobb is a thin layer on top of bolt
2. if you understand how bolt works you will get bobb
3. review code in view_handlers.go or updt_handlers.go to see how bobb works

### Read - [info folder](info)
* info/install.txt - install, verify, use instructions
* info/api.txt - details operations
* long-readme.md - full info on bobb
