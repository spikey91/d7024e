package d7024e

import (
	"log"
	"time"
	"sync"
)

type Kademlia struct {
	rt *RoutingTable
	network *Network
	mux sync.Mutex
}

func NewKademlia (rt *RoutingTable, network *Network) Kademlia {
	return Kademlia{rt, network, sync.Mutex{}}
}

const Alpha int = 2;
const K int = 3;



func (kademlia *Kademlia) LookupContact(targetID *KademliaID) []Contact {
	//we have to mutex lock this procedure as this might be called from republish.
	kademlia.mux.Lock()

	//selected the alpha closest from our own routing table to the target
	myKClosest := kademlia.rt.FindClosestContacts(targetID, K)

	kClosest := make([]Contact, 0)
	kClosest = append(kClosest, myKClosest...)

	toBeQueried := make([]Contact, 0)
	toBeQueried = append(toBeQueried, kClosest...)

	if len(kClosest) > Alpha {	//if there are more than alpha entries.
		toBeQueried = append(toBeQueried, kClosest[0:Alpha]...)
	}
	
	queriedContacts := make([]Contact, 0)

	kClosest = kademlia.NodeLookup(toBeQueried, kClosest, queriedContacts, targetID)

	defer kademlia.mux.Unlock()		//release the mutex lock after the result is returned.
	return kClosest;
}

func (kademlia *Kademlia) NodeLookup(toBeQueried []Contact, kClosest []Contact, queriedContacts []Contact, targetID *KademliaID) []Contact {

	//base case
	if len(toBeQueried) == 0 {
		return kClosest;
	}

	for i := range toBeQueried {
		go kademlia.network.SendFindNodeMessage(toBeQueried[i].Address, targetID.String())
		queriedContacts = append(queriedContacts, toBeQueried[i])

	}

	toBeQueried = ClearContactSlice(toBeQueried)
	currentKClosest := kClosest
	roundSuccessful := false
	for {
	    select {
	        case <-time.After(time.Millisecond * 500):
		    	log.Println("timeout!!")
		    	break;

	    	case c := <-kademlia.network.ReturnedContacts:

	    	    //check that c is not already in currentKClosest.
				if ContainsContact(currentKClosest, c) == true {
					log.Println("contact" + c.Address + "already in currentKClosest!")
					continue;

				//if currentKClosest holds k items in the array add the contact to k-closest
				} else if len(currentKClosest) >= K {  
		    	    currentKClosest = InsertContactSortedDistTarget(c, currentKClosest, targetID)
		    	    
		    	    //if at least one contact was not inserted on the last index, means that it was of closer distance than some other contact in currentKClosest to our target.
		    	    if currentKClosest[K].ID.String() != c.ID.String() {
		    	    	roundSuccessful = true;
		    	    	log.Println("contact " + c.Address + " was added!")
		    	    }

		    	    //and strip the list to K items
		    	    currentKClosest = currentKClosest[0:K]

		    	//if currentKClosest holds less than K items
				} else if len(currentKClosest) < K {
					
		    	    //add the contact to k-closest
		    	    currentKClosest = InsertContactSortedDistTarget(c, currentKClosest, targetID)
		    	    roundSuccessful = true;
		    	    log.Println("contact " + c.Address + " was added!")
				}
				continue;		//go back to the select case.
			}
		break;	//break out of the outer for-loop.
	}


	limit := Alpha
	if roundSuccessful == false {
		limit = K
	}

	contactsToQuery := 0
	for i := range currentKClosest {
		alreadyQueried := false;
		currentContact := currentKClosest[i];

		if contactsToQuery >= limit {
			break;
		}

		if ContainsContact(queriedContacts, &currentContact) == true {
			alreadyQueried = true;
		}

		if alreadyQueried == false{
			contactToBeAdded := NewContact(NewKademliaID(currentContact.ID.String()), currentContact.Address)
			toBeQueried = append(toBeQueried, contactToBeAdded)
			contactsToQuery ++;
		}
		
	}

	return kademlia.NodeLookup(toBeQueried, currentKClosest, queriedContacts, targetID)
}


func (kademlia *Kademlia) LookupData(targetKey *KademliaID) []byte {

	//selected the alpha closest from our own routing table to the target
	myKClosest := kademlia.rt.FindClosestContacts(targetKey, K)

	kClosest := make([]Contact, 0)
	kClosest = append(kClosest, myKClosest...)

	toBeQueried := make([]Contact, 0)
	toBeQueried = append(toBeQueried, kClosest...)

	if len(kClosest) > Alpha {	//if there are more than alpha entries.
		toBeQueried = append(toBeQueried, kClosest[0:Alpha]...)
	}
	
	queriedContacts := make([]Contact, 0)

	data := kademlia.ValueLookup(toBeQueried, kClosest, queriedContacts, targetKey)
	return data;
} 

func (kademlia *Kademlia) ValueLookup(toBeQueried []Contact, kClosest []Contact, queriedContacts []Contact, targetID *KademliaID) []byte {

	//base case, when no contacts are left to query we return an empty result.
	if len(toBeQueried) == 0 {
		log.Println("returning nil.-..")
		return nil;
	}

	for i := range toBeQueried {
		go kademlia.network.SendFindDataMessage(toBeQueried[i].Address, targetID.String())		//we will send find data messages instead of find node
		queriedContacts = append(queriedContacts, toBeQueried[i])
	}

	fileReturned := false;
	currentKClosest := kClosest
	var returnedFilePacket *FilePacket;
	roundSuccessful := false;
	for {
	    select {
	        case <-time.After(time.Millisecond * 2000):
		    	log.Println("timeout!!")
		    	break;

	        case filePacket := <-kademlia.network.ReturnedPacketFiles:
	        	//if the file is returned
		    	log.Println("File returned: " + filePacket.ID + " from node: " + filePacket.SourceNodeID)
		    	fileReturned = true;
		    	returnedFilePacket = filePacket;
		    	continue;

	    	case c := <-kademlia.network.ReturnedContacts:

	    	    //check that c is not already in currentKClosest.
				if ContainsContact(currentKClosest, c) == true {
					log.Println("contact" + c.Address + "already in currentKClosest!")
					continue;

				//if currentKClosest holds k items in the array add the contact to k-closest
				} else if len(currentKClosest) >= K {  
		    	    currentKClosest = InsertContactSortedDistTarget(c, currentKClosest, targetID)
		    	    
		    	    //if at least one contact was not inserted on the last index, means that it was of closer distance than some other contact in currentKClosest to our target.
		    	    if currentKClosest[K].ID.String() != c.ID.String() {
		    	    	roundSuccessful = true;
		    	    	log.Println("contact " + c.Address + " was added!")
		    	    }

		    	    //and strip the list to K items
		    	    currentKClosest = currentKClosest[0:K]

		    	//if currentKClosest holds less than K items
				} else if len(currentKClosest) < K {
					
		    	    //add the contact to k-closest
		    	    currentKClosest = InsertContactSortedDistTarget(c, currentKClosest, targetID)
		    	    roundSuccessful = true;
		    	    log.Println("contact " + c.Address + " was added!")
				}
				continue;		//go back to the select case.
			}
		break;	//break out of the outer for-loop.
	}

	if fileReturned == true {
		//For caching reasons, find the closest observed contact that did not return the file:
		for i := range kClosest {
			if kClosest[i].ID.String() == returnedFilePacket.SourceNodeID {
				continue;
			}
			//send a store request to that node.
			fileToBeStored := NewFile(returnedFilePacket.ID, returnedFilePacket.Data)
			defer kademlia.network.SendStoreMessage(kClosest[i].Address, &fileToBeStored)
			log.Println("Store request sent to: " + kClosest[i].Address)
			break;
		}

		//lastly, return the data itself and exit the function.
		return returnedFilePacket.Data;
	}

	toBeQueried = ClearContactSlice(toBeQueried)

	limit := Alpha
	if roundSuccessful == false {
		limit = K
	}

	contactsToQuery := 0
	for i := range currentKClosest {
		alreadyQueried := false;
		currentContact := currentKClosest[i];

		if contactsToQuery >= limit {
			break;
		}

		if ContainsContact(queriedContacts, &currentContact) == true {
			alreadyQueried = true;
		}

		if alreadyQueried == false{
			contactToBeAdded := NewContact(NewKademliaID(currentContact.ID.String()), currentContact.Address)
			toBeQueried = append(toBeQueried, contactToBeAdded)
			contactsToQuery ++;
		}
		
	}

	return kademlia.ValueLookup(toBeQueried, currentKClosest, queriedContacts, targetID)
}


func (kademlia *Kademlia) Store(data []byte) *KademliaID{
	fileToBeAdded := NewFile("", data)

	//find the closest contacts
	kClosest := kademlia.LookupContact(fileToBeAdded.Key)
	PrintContactList(kClosest)
	//send them store RPCs
	for i := range kClosest {
		kademlia.network.SendStoreMessage(kClosest[i].Address, &fileToBeAdded)
	}

	//the original publisher will re-publish this file periodically.
	go kademlia.RePublish(data, fileToBeAdded.Key)

	return fileToBeAdded.Key;
}

func (kademlia *Kademlia) RePublish(data []byte, fileKey *KademliaID) {
    fileToRePublished := NewFile(fileKey.String(), data)

    republishTimer := time.Second * 60
    tickChan := time.NewTicker(republishTimer).C

    for {
    	select {
    		case <- tickChan:
    			log.Println("Re-publishing: " + fileKey.String() + "...")
			    //find the closest contacts
				kClosest := kademlia.LookupContact(fileToRePublished.Key)
				PrintContactList(kClosest)
				//send them store RPCs
				for i := range kClosest {
					kademlia.network.SendStoreMessage(kClosest[i].Address, &fileToRePublished)
				}
    	}
    }
}

func ContainsContact(contacts []Contact, contact *Contact) bool {
    for i := range contacts {
        if contact.ID.String() == contacts[i].ID.String() {
            return true
        }
    }
    return false
}

func InsertContactSortedDistTarget(contact *Contact, list []Contact, target *KademliaID) []Contact {
	/* This function will insert a contact in a list, the list is sorted on distance to target. */

	//find the right index
	index := len(list)	//initialize it as the last index.
	distanceNewContact := contact.ID.CalcDistance(target)

	for i := range list {
		currentContact := &list[i]
		distanceCurrentContact := currentContact.ID.CalcDistance(target)
		//currentContact.CalcDistance(target)

		if distanceNewContact.Less(distanceCurrentContact) {		//kClosest is sorted on distance to target node.
			index = i;
			break;
		} 
	}

	s := append(list, list[0])
	copy(s[index+1:], s[index:])
	s[index] = *contact
	return s
}

func ClearContactSlice(list []Contact) []Contact {
	return list[:0]
}