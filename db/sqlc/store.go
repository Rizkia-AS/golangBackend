package db

import (
	"context"
	"database/sql"
	"fmt"
	// "sync"
)

// Store provide all functions to execute db queries and transaction
type Store interface {
	TransferTxV2(ctx context.Context, arg TransferTxParams) (TransferTxResult, error)
	Querier // embed interface QUerier membuat store interface memiliki semua function yang dimiliki oleh interface querier
}

// SQLStore menyediakan semua function" untuk mengeksekusi db queris dan transaction
// SQLStore provides all functions to executes SQL. queries and transaction
type SQLStore struct {
	// mutex sync.Mutex
	// extending queries, dengan embeding queries ke struct store maka setiap function" yang ada disediakan pada queris akan tersedia di struct store ini dan kita bisa menambahkan function" tsb ke struct transaction yang baru
	*Queries
	// untuk melakukannya kita juga perlu Store untuk memiliki object db, karna object tsb diperlukan untuk membuat db transaction baru
	db *sql.DB
}

// membuat new store
func NewStore(db *sql.DB) Store {
	// membuat newstore object dan mengembalikannya
	return &SQLStore{
		db: db,
		// new membuat dan mengembalikan object Queries
		Queries: New(db),
	}
}

// mengeksekusi function yang ada dalam database
func (store *SQLStore) execTx(ctx context.Context, fn func(*Queries) error) error {
	// mulai transaction baru
	// &sql.TxOptions{} membolehkan kita untuk melakukan custom level dari isolation. jika tidak diterapkan maka level nya akan default sesuai dengan jenis database yg digukanan. untuk postgres levelnya write commited
	// tx,err := store.db.BeginTx(ctx, &sql.TxOptions{})
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := New(tx)
	err = fn(q)

	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			// kalau error pada tx terjadi dan terjadi juga error pada proses rollback
			return fmt.Errorf("tx err: %v, rb err : %v", err, rbErr)
		}
		// jika rollback sukses, kita hanya perlu mengembalikan error pada tx saja
		return err
	}

	// jika tidak ada kendala dan seluruh operasi sukses maka lakukan commit tx
	// kembalikan error pada caller jika ada
	return tx.Commit()

}

// TransferTxParams berisi parameters yang dibutuhkan untuk transaction transfer

type TransferTxResult struct {
	Transfer    Transfer `json:"transfer"`
	FromAccount Account  `json:"from_account"`
	ToAccount   Account  `json:"to_account"`
	FromEntry   Entry    `json:"from_entry`
	ToEntry     Entry    `json:"to_entry"`
}

type TransferTxParams struct {
	FromAccountID int64  `json:"from_account_id"`
	ToAccountID   int64  `json:"to_account_id"`
	Amount        int64  `json:"amount"`
	Description   string `json:"description"`
}

// var txKey = struct{}{}  // just for debugging purpose

// TransferTx menjalankan proses transfer dari satu akun ke akun lainnya
// buat transfer record, entries dan mengupdate balance dari setiap akun
func (store *SQLStore) TransferTx(ctx context.Context, arg TransferTxParams) (TransferTxResult, error) {
	// store.mutex.Lock()
	// defer store.mutex.Unlock()

	var result TransferTxResult

	// create and run db tx
	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// txName := ctx.Value(txKey)  // just for debugging purpose

		// fmt.Println(txName, "create transfer") // just for debugging purpose
		result.Transfer, err = q.CreateTransfer(ctx, CreateTransferParams{
			FromAccountID: arg.FromAccountID,
			ToAccountID:   arg.ToAccountID,
			Amount:        arg.Amount,
			Description:   arg.Description,
		})

		if err != nil {
			return err
		}

		// fmt.Println(txName, "create entry for sender") // just for debugging purpose
		result.FromEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID:   arg.FromAccountID,
			Amount:      -arg.Amount,
			Description: arg.Description,
		})

		if err != nil {
			return err
		}

		// fmt.Println(txName, "create entry for receiver") // just for debugging purpose
		result.ToEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID:   arg.ToAccountID,
			Amount:      arg.Amount,
			Description: arg.Description,
		})

		if err != nil {
			return err
		}

		// TODO: update account's balance
		// fmt.Println(txName, "get sender account") // just for debugging purpose
		senderAccount, err := q.GetAccountForUpdate(ctx, arg.FromAccountID)
		if err != nil {
			return err
		}

		// fmt.Println(txName, "update sender account") // just for debugging purpose
		result.FromAccount, err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      arg.FromAccountID,
			Balance: senderAccount.Balance - arg.Amount,
		})

		if err != nil {
			return err
		}

		// fmt.Println(txName, "get receiver account") // just for debugging purpose
		receiverAccount, err := q.GetAccountForUpdate(ctx, arg.ToAccountID)
		if err != nil {
			return err
		}

		// fmt.Println(txName, "update receiver account") // just for debugging purpose
		result.ToAccount, err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      arg.ToAccountID,
			Balance: receiverAccount.Balance + arg.Amount,
		})

		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}

// TransferTx yang sudah menghandle adanya deadlock
func (store *SQLStore) TransferTxV2(ctx context.Context, arg TransferTxParams) (TransferTxResult, error) {
	var result TransferTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Transfer, err = q.CreateTransfer(ctx, CreateTransferParams{
			FromAccountID: arg.FromAccountID,
			ToAccountID:   arg.ToAccountID,
			Amount:        arg.Amount,
			Description:   arg.Description,
		})

		if err != nil {
			return err
		}

		result.FromEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID:   arg.FromAccountID,
			Amount:      -arg.Amount,
			Description: arg.Description,
		})

		if err != nil {
			return err
		}

		result.ToEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID:   arg.ToAccountID,
			Amount:      arg.Amount,
			Description: arg.Description,
		})

		if err != nil {
			return err
		}

		// untuk menghindari deadlock kita bisa memastikan agar transaction yang dijalankan selalu dari id account yang paling kecil terlebih dahulu
		if arg.FromAccountID < arg.ToAccountID {
			result.FromAccount, result.ToAccount, err = addMoney(ctx, q, arg.FromAccountID, -arg.Amount, arg.ToAccountID, arg.Amount)
		} else {
			result.ToAccount, result.FromAccount, err = addMoney(ctx, q, arg.ToAccountID, arg.Amount, arg.FromAccountID, -arg.Amount)

		}

		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}

func addMoney(
	ctx context.Context,
	q *Queries,
	account1ID,
	amount1,
	account2ID,
	amount2 int64,
) (account1, account2 Account, err error) {
	account1, err = q.UpdateAccountBalance(ctx, UpdateAccountBalanceParams{
		Amount: amount1,
		ID:     account1ID,
	})
	if err != nil {
		// karna account1,err = q.UpdateAcc... sama dengan nama variable yang direturn oleh function ini
		// maka return dibawah berarti return account1, account 2, err
		return
	}

	account2, err = q.UpdateAccountBalance(ctx, UpdateAccountBalanceParams{
		Amount: amount2,
		ID:     account2ID,
	})

	return
}
