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

func GetPostPrice(uploader string, postId string, tx string) (int64, error) {
	query := fmt.Sprintf(`{
		transactions(
			owners: ["%s"],
			recipients: ["%s"]
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
					timestamp
				}
			}
		}
	}`, uploader, common.ACTIVATION_ADDRESS, common.TX_APP_NAME, common.TX_APP_VERSION, common.TX_TYPE_SET_PRICE, postId)

	jsonData, err := QueryArweave(query)
	if err != nil {
		return 0, fmt.Errorf("query error: %w", err)
	}

	var setPriceResults common.ArQueryResult
	err = json.Unmarshal(jsonData, &setPriceResults)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	if len(setPriceResults.Data.Transactions.Edges) == 0 {
		return 0, fmt.Errorf("no price set for transaction")
	}

	// Get the payment transaction timestamp and quantity
	paymentQuery := fmt.Sprintf(`{
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
					block {
						timestamp
					}
					quantity {
						winston
					}
				}
			}
		}
	}`, uploader, common.TX_APP_NAME, common.TX_APP_VERSION, common.TX_TYPE_PAYMENT, tx)

	paymentData, err := QueryArweave(paymentQuery)
	if err != nil {
		return 0, fmt.Errorf("payment query error: %w", err)
	}

	var paymentResult common.ArQueryResult
	err = json.Unmarshal(paymentData, &paymentResult)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling payment JSON: %w", err)
	}

	if len(paymentResult.Data.Transactions.Edges) == 0 {
		return 0, fmt.Errorf("no payment transaction found")
	}

	paymentTimestamp := paymentResult.Data.Transactions.Edges[0].Node.Block.Timestamp
	paymentQuantity, err := strconv.ParseInt(paymentResult.Data.Transactions.Edges[0].Node.Quantity.Winston, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing payment amount: %w", err)
	}

	// Find the most recent price set before the payment that matches the payment amount
	var validPrice int64 = 0
	for _, edge := range setPriceResults.Data.Transactions.Edges {
		if edge.Node.Block.Timestamp <= paymentTimestamp {
			priceAmount, err := strconv.ParseInt(edge.Node.Quantity.Winston, 10, 64)
			if err != nil {
				continue
			}
			if paymentQuantity >= priceAmount {
				validPrice = priceAmount
				break
			}
		}
	}

	if validPrice == 0 {
		return 0, fmt.Errorf("no valid price found before payment")
	}

	return validPrice, nil
}

func CheckPayment(sender string, tx string, uploader string, postId string) (bool, error) {
	// Get the required price first - use wallet (content owner) as sender
	requiredPrice, err := GetPostPrice(uploader, postId, tx)
	if err != nil {
		return false, nil
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
	paidAmount, err := strconv.ParseInt(result.Data.Transactions.Edges[0].Node.Quantity.Winston, 10, 64)
	if err != nil {
		return false, fmt.Errorf("error parsing payment amount: %w", err)
	}

	return paidAmount >= requiredPrice, nil
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
