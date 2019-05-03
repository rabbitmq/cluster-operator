package rabbithutch

func (r *rabbitHutch) UserList() ([]string, error) {
	userInfoList, err := r.client.ListUsers()
	if err != nil {
		return nil, err
	}

	userNames := make([]string, 0, len(userInfoList))

	for _, user := range userInfoList {
		userNames = append(userNames, user.Name)
	}

	return userNames, nil
}
