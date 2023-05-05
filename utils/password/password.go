package password

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword compute bycript and return the bycript has of the passsword
func HashedPassword(password string) (string, error) {
	// bcrypt.GenerateFromPassword(convert password string into byte slice, )
	hashPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("gagal untuk meng hash password : %w", err)
	}
	// convert hash password dari yang berbentuk byte slice menjadi string kembali. dan kembalikan error dengan nilai nil
	return string(hashPassword), nil
}

func CheckPassword(password string, hashPassword string) error {
	// mengambil data cost dan salt dari hashpassword yang disimpan didatabase, lalu dengan cost dan salt tersebut kita jadikan patokan untuk mengubah password yang baru dikirimkan oleh user menjadi hash baru, karena dua duanya sudah berbentuk hash maka kini bisa dikomparasikan
	return bcrypt.CompareHashAndPassword([]byte(hashPassword), []byte(password))
}
