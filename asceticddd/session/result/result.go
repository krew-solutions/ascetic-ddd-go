package result

import "errors"

func NewResultImp(lastInsertId, rowsAffected int64) ResultImp {
	return ResultImp{lastInsertId, rowsAffected}
}

type ResultImp struct {
	lastInsertId int64
	rowsAffected int64
}

func (r ResultImp) LastInsertId() (int64, error) {
	if r.rowsAffected == 0 {
		return r.lastInsertId, nil
	} else {
		return 0, errors.New("LastInsertId is not supported by this driver")
	}
}

func (r ResultImp) RowsAffected() (int64, error) {
	if r.lastInsertId == 0 {
		return r.rowsAffected, nil
	} else {
		return 0, errors.New("RowsAffected is not supported by INSERT command")
	}
}
