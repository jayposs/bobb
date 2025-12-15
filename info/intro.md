## Bobb - JSON database built on [Bolt/Bbolt](https://github.com/etcd-io/bbolt)

Bobb attempts to find a good balance of small code size, simplicity, speed, and usefulness. It is a thin layer on top of the key-value data store, Bolt. Understanding how Bolt works is important. For example, when using a key/index range to limit record input, requests can run at hyper speed. Bobb is easy to use, but places a lot of responsibility on the developer. 

### Features
* Http Server allows multiple programs to simultaneously access the same db
* Client package provides easy to use interface
* Secondary Indexes
* Queries with multiple search criteria and sort parameters
* Simple Joins 
  
**[bobb on github](https://github.com/jayposs/bobb)** 
