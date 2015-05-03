# flux
fbp style socket structure

##Sockets
 - Push
 Push builds on top of the socket structure where every data emitted is immediately sent to the listener lists
 
 - Pull
 Pull builds on top of the socket structure and emits data into a buffered channel which then can be pull into the listeners 
