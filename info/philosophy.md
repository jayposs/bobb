## Code Philosophy  

My goal is to provide code that can be owned by each developer using it. I think this goal is possible by following several guidelines.

* Simplicity is the guiding force over maximum functionality
* Shallow design makes it easy to find what you're looking for
* Straight forward code is priority over minimum code size and use of "fancy" techniques
* Don't try to catch and handle every possible error 
* Document changes so they can be evaluated and incorporated if desired

Package software never provides everything needed, no matter how many features are included. It is also usually too large and complex for the developer to consider adding their own features. Of course many projects will need the functionality big name tools deliver. Products like boltdb have their place and Bobb tries to expand its number of use cases. 
 
Error handling is a tough nut to crack. Our code can become so cluttered with error handling that errors are more likely just because it's difficult to follow the core logic. My opinion is most errors are caught because the results are incorrect. Testing will usually find these problems. Of course there are situations where more detailed error detection is needed. Developers of these projects should feel able to add it based on their requirements.

With Bobb, the code is very shallow and easy to follow. Adding a new feature typically involves:
* adding or changing request type 
* adding or changing request handler function
* adding routing code to bobb_server.go if required 

All request handlers return the same response type which simplifies things a lot.