## Quick Start

1. clone git repo: git clone https://github.com/jayposs/bobb.git  
2. cd to bobb directory    
3. run go mod tidy - should get bbolt and fastjson
4. cd to server sub directory
5. confirm bobb_settings.json are ok
6. go run bobb_server.go  
   verify "waiting for requests ..." is displayed    
7. open new terminal window (or put server to background**)
8. cd ../demo   
9. go run demo.go  
   verify "Demo Pgm Finished Successfully" is displayed  

****To put server to background**  
1. ctrl-z  
2. bg  
3. jobs -l (should show running server)  
  
see linux_notes.txt for more info    

### Suggestion  
Spin up a cloud server to try out.  
DigitalOcean $4/mo droplet works fine.  
See info/server_setup.txt for steps. 