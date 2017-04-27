package gmaj

import (
	"errors"
	"math/rand"
	"github.com/r-medina/gmaj/gmajpb"
	"fmt"
	"golang.org/x/net/context"
)

var (
	emptyRemote = &gmajpb.Node{}
	mt          = &gmajpb.MT{}
)

// GetPredecessor gets the predecessor on the node.
func (node *Node) GetPredecessor(context.Context, *gmajpb.MT) (*gmajpb.Node, error) {
	node.predMtx.RLock()
	pred := node.predecessor
	node.predMtx.RUnlock()

	if pred == nil {
		return emptyRemote, nil
	}

	return pred, nil
}

// GetSuccessor gets the successor on the node..
func (node *Node) GetSuccessor(context.Context, *gmajpb.MT) (*gmajpb.Node, error) {
	node.succMtx.RLock()
	succ := node.successor
	node.succMtx.RUnlock()

	if succ == nil {
		return emptyRemote, nil
	}

	return succ, nil
}

// SetPredecessor sets the predecessor on the node.
func (node *Node) SetPredecessor(
	ctx context.Context, pred *gmajpb.Node,
) (*gmajpb.MT, error) {
	node.predMtx.Lock()
	node.predecessor = pred
	node.predMtx.Unlock()

	return mt, nil
}

// SetPredecessor2 sets the predecessor2 on the node.
// func (node *Node) SetPredecessor2(
// 	ctx context.Context, pred2 *gmajpb.Node,
// ) (*gmajpb.MT, error) {
// 	node.pred2Mtx.Lock()
// 	node.predecessor2 = pred2
// 	node.pred2Mtx.Unlock()

// 	return mt, nil
// }

// SetSuccessor sets the successor on the node.
func (node *Node) SetSuccessor(
	ctx context.Context, succ *gmajpb.Node,
) (*gmajpb.MT, error) {
	node.succMtx.Lock()
	node.successor = succ
	node.succMtx.Unlock()

	return mt, nil
}

// SetSuccessor2 sets the successor2 on the node.
func (node *Node) SetSuccessor2(
	ctx context.Context, succ2 *gmajpb.Node,
) (*gmajpb.MT, error) {
	node.succ2Mtx.Lock()
	node.successor2 = succ2
	node.succ2Mtx.Unlock()

	return mt, nil
}


// Notify is called when remoteNode thinks it's our predecessor.
func (node *Node) Notify(
	ctx context.Context, remoteNode *gmajpb.Node,
) (*gmajpb.MT, error) {
	node.notify(remoteNode)

	// If node.Predecessor is nil at this point, we were trying to notify
	// ourselves. Otherwise, to succeed, we must check that the successor
	// was correctly updated.
	node.predMtx.Lock()
	defer node.predMtx.Unlock()
	if node.predecessor != nil && !idsEqual(node.predecessor.Id, remoteNode.Id) {
		return nil, errors.New("gmaj: node is not predecesspr")
	}

	return mt, nil
}

// ClosestPrecedingFinger will find the closest preceding entry in the finger
// table based on the id.
func (node *Node) ClosestPrecedingFinger(
	ctx context.Context, id *gmajpb.ID,
) (*gmajpb.Node, error) {
	remoteNode := node.closestPrecedingFinger(id.Id)
	if remoteNode == nil {
		return nil, errors.New("gmaj: no closest preceding finger")
	}

	return remoteNode, nil
}

// FindSuccessor finds the successor, error if nil.
func (node *Node) FindSuccessor(
	ctx context.Context, id *gmajpb.ID,
) (*gmajpb.Node, error) {
	succ, err := node.findSuccessor(id.Id)
	if err != nil {
		return emptyRemote, err
	}

	if succ == nil {
		return nil, errors.New("gmaj: cannot find successor")
	}

	return succ, nil
}

// GetKey returns the value of the key requested at the node.
func (node *Node) GetKey(ctx context.Context, key *gmajpb.Key) (*gmajpb.Val, error) {
	node.lbMtx.RLock()
	defer node.lbMtx.RUnlock()

	rand_no := rand.Float64()

	if rand_no < 0.5 {
		fmt.Printf("Being routed to Backup\n")
		val, err := node.getBackupKeyRPC(node.successor, key.Key)
		if err != nil {
			return nil, err
		}
		return &gmajpb.Val{Val: val}, nil
	}
	fmt.Printf("Received get request for key: %s\n", key.Key)
	val, err := node.getKey(key.Key)
	if err != nil {
		return nil, err
	}

	return &gmajpb.Val{Val: val}, nil
}

// GetKey returns the value of the key requested at the node.
func (node *Node) GetBackupKey(ctx context.Context, key *gmajpb.Key) (*gmajpb.Val, error) {
	fmt.Printf("Received get request for key: %s from backup\n", key.Key)
	val, err := node.getBackupKey(key.Key)
	if err != nil {
		return nil, err
	}

	return &gmajpb.Val{Val: val}, nil
}

// GetKey returns the value of the key requested at the node.
func (node *Node) RequestAllData(ctx context.Context, key *gmajpb.Key) (*gmajpb.Val, error) {

	node.dsMtx.RLock()
	for k, val := range node.datastore {
		node.putKeyValBackupRPC(node.successor, k, val)
	}
	node.dsMtx.RUnlock()

	return &gmajpb.Val{Val: nil}, nil
}


// PutKeyVal stores a key value pair on the node.
func (node *Node) PutKeyVal(ctx context.Context, kv *gmajpb.KeyVal) (*gmajpb.MT, error) {
	node.lbMtx.Lock()
	defer node.lbMtx.Unlock()

	if err := node.putKeyVal(kv); err != nil {
		return nil, err
	}

	node.putKeyValBackupRPC(node.successor, kv.Key, kv.Val)

	return mt, nil
}

// PutKeyVal stores a key value pair on the node.
func (node *Node) PutKeyValBackup(ctx context.Context, kv *gmajpb.KeyVal) (*gmajpb.MT, error) {
	fmt.Printf("Key: %s Val: %s pair added to backup\n", kv.Key, kv.Val)
	if err := node.putKeyValBackup(kv); err != nil {
		return nil, err
	}

	return mt, nil
}

// PutKeyVal stores a key value pair on the node.
func (node *Node) RemoveKeyValBackup(ctx context.Context, kv *gmajpb.KeyVal) (*gmajpb.MT, error) {
	if err := node.removeKeyValBackup(kv); err != nil {
		return nil, err
	}

	return mt, nil
}

// TransferKeys transfers the appropriate keys on this node
// to the remote node specified in the request.
func (node *Node) TransferKeys(
	ctx context.Context, tmsg *gmajpb.TransferKeysReq,
) (*gmajpb.MT, error) {
	if err := node.transferKeys(tmsg.FromId, tmsg.ToNode); err != nil {
		return nil, err
	}

	return mt, nil
}
