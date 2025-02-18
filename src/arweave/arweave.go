package arweave

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/acsermely/veracy.server/src/common"
)

func Query(query string) ([]byte, error) {

	jsonData := map[string]string{
		"query": query,
	}
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return nil, err
	}
	resp, err := http.Post(common.BUNDLER_URL+"/graphql", common.TX_APP_CONTENT_TYPE, bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println("Error sending query:", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil, err
	}

	return body, nil
}

func QueryArweave(query string) ([]byte, error) {
	jsonData := map[string]string{
		"query": query,
	}
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return nil, err
	}
	resp, err := http.Post(common.ARWEAVE_URL+"/graphql", common.TX_APP_CONTENT_TYPE, bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println("Error sending query:", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil, err
	}

	return body, nil
}

func GetPostPrice(tx string) (int32, error) {
	query := fmt.Sprintf(`{
		transactions(
			tags: [
				{ name: "App-Name", values: ["%s"]},
				{ name: "Version", values: ["%s"]},
				{ name: "Type", values: ["%s"]},
				{ name: "Target", values: ["%s"]}
			]
		)
		{
			edges {
				node {
					id
					quantity {
						winston
					}
				}
			}
		}
	}`, common.TX_APP_NAME, common.TX_APP_VERSION, common.TX_TYPE_SET_PRICE, tx)

	jsonData, err := QueryArweave(query)
	if err != nil {
		return 0, fmt.Errorf("query error: %w", err)
	}

	var result common.ArQueryResult
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	if len(result.Data.Transactions.Edges) == 0 {
		return 0, fmt.Errorf("no price set for transaction")
	}

	// Convert winston string to int32
	winston, err := strconv.ParseInt(result.Data.Transactions.Edges[0].Node.Quantity.Winston, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("error parsing winston amount: %w", err)
	}

	return int32(winston), nil
}

func CheckPayment(sender string, tx string) (bool, error) {
	// Get the required price first
	requiredPrice, err := GetPostPrice(tx)
	if err != nil {
		return false, fmt.Errorf("error getting post price: %w", err)
	}

	query := fmt.Sprintf(`{
		transactions(
			owners: ["%s"],
			tags: [
				{ name: "App-Name", values: ["%s"]},
				{ name: "Version", values: ["%s"]},
				{ name: "Type", values: ["%s"]},
				{ name: "Target", values: ["%s"]}
			]
		)
		{
			edges {
				node {
					id
					quantity {
						winston
					}
				}
			}
		}
	}`, sender, common.TX_APP_NAME, common.TX_APP_VERSION, common.TX_TYPE_PAYMENT, tx)

	jsonData, err := QueryArweave(query)
	if err != nil {
		return false, fmt.Errorf("query error: %w", err)
	}

	var result common.ArQueryResult
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return false, fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	if len(result.Data.Transactions.Edges) == 0 {
		return false, nil
	}

	// Check if payment amount matches required price
	paidAmount, err := strconv.ParseInt(result.Data.Transactions.Edges[0].Node.Quantity.Winston, 10, 32)
	if err != nil {
		return false, fmt.Errorf("error parsing payment amount: %w", err)
	}

	return int32(paidAmount) >= requiredPrice, nil
}

func IsDataPrivate(fullId string, tx string) (bool, error) {
	postData, err := getTxById(tx)
	if err != nil {
		return false, err
	}
	for _, content := range postData.Content {
		if content.Data == fullId {
			isPrivate := content.Privacy == common.TX_POST_PRIVACY_PRIVATE
			return isPrivate, nil
		}
	}
	return false, fmt.Errorf("ID not found")
}

func getTxById(txId string) (*common.Post, error) {
	response, err := http.Get(fmt.Sprintf("%s/%s", common.BUNDLER_URL, txId))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var post common.Post
	if err := json.NewDecoder(response.Body).Decode(&post); err != nil {
		return nil, err
	}
	post.ID = txId
	return &post, nil
}
