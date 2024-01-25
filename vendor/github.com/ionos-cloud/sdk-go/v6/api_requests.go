/*
 * CLOUD API
 *
 * IONOS Enterprise-grade Infrastructure as a Service (IaaS) solutions can be managed through the Cloud API, in addition or as an alternative to the \"Data Center Designer\" (DCD) browser-based tool.    Both methods employ consistent concepts and features, deliver similar power and flexibility, and can be used to perform a multitude of management tasks, including adding servers, volumes, configuring networks, and so on.
 *
 * API version: 6.0
 */

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package ionoscloud

import (
	_context "context"
	"fmt"
	"io"
	_nethttp "net/http"
	_neturl "net/url"
	"strings"
)

// Linger please
var (
	_ _context.Context
)

// RequestsApiService RequestsApi service
type RequestsApiService service

type ApiRequestsFindByIdRequest struct {
	ctx             _context.Context
	ApiService      *RequestsApiService
	requestId       string
	pretty          *bool
	depth           *int32
	xContractNumber *int32
}

func (r ApiRequestsFindByIdRequest) Pretty(pretty bool) ApiRequestsFindByIdRequest {
	r.pretty = &pretty
	return r
}
func (r ApiRequestsFindByIdRequest) Depth(depth int32) ApiRequestsFindByIdRequest {
	r.depth = &depth
	return r
}
func (r ApiRequestsFindByIdRequest) XContractNumber(xContractNumber int32) ApiRequestsFindByIdRequest {
	r.xContractNumber = &xContractNumber
	return r
}

func (r ApiRequestsFindByIdRequest) Execute() (Request, *APIResponse, error) {
	return r.ApiService.RequestsFindByIdExecute(r)
}

/*
 * RequestsFindById Retrieve requests
 * Retrieve the properties of the specified request.
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param requestId The unique ID of the request.
 * @return ApiRequestsFindByIdRequest
 */
func (a *RequestsApiService) RequestsFindById(ctx _context.Context, requestId string) ApiRequestsFindByIdRequest {
	return ApiRequestsFindByIdRequest{
		ApiService: a,
		ctx:        ctx,
		requestId:  requestId,
	}
}

/*
 * Execute executes the request
 * @return Request
 */
func (a *RequestsApiService) RequestsFindByIdExecute(r ApiRequestsFindByIdRequest) (Request, *APIResponse, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodGet
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  Request
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "RequestsApiService.RequestsFindById")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/requests/{requestId}"
	localVarPath = strings.Replace(localVarPath, "{"+"requestId"+"}", _neturl.PathEscape(parameterToString(r.requestId, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.pretty != nil {
		localVarQueryParams.Add("pretty", parameterToString(*r.pretty, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("pretty")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("pretty", parameterToString(true, ""))
		}
	}
	if r.depth != nil {
		localVarQueryParams.Add("depth", parameterToString(*r.depth, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("depth")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("depth", parameterToString(0, ""))
		}
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xContractNumber != nil {
		localVarHeaderParams["X-Contract-Number"] = parameterToString(*r.xContractNumber, "")
	}
	if r.ctx != nil {
		// API Key Authentication
		if auth, ok := r.ctx.Value(ContextAPIKeys).(map[string]APIKey); ok {
			if apiKey, ok := auth["Token Authentication"]; ok {
				var key string
				if apiKey.Prefix != "" {
					key = apiKey.Prefix + " " + apiKey.Key
				} else {
					key = apiKey.Key
				}
				localVarHeaderParams["Authorization"] = key
			}
		}
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, httpRequestTime, err := a.client.callAPI(req)

	localVarAPIResponse := &APIResponse{
		Response:    localVarHTTPResponse,
		Method:      localVarHTTPMethod,
		RequestURL:  localVarPath,
		RequestTime: httpRequestTime,
		Operation:   "RequestsFindById",
	}

	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarAPIResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarAPIResponse.Payload = localVarBody
	if err != nil {
		return localVarReturnValue, localVarAPIResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			statusCode: localVarHTTPResponse.StatusCode,
			body:       localVarBody,
			error:      fmt.Sprintf(FormatStringErr, localVarHTTPResponse.Status, string(localVarBody)),
		}
		var v Error
		err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
		if err != nil {
			newErr.error = fmt.Sprintf(FormatStringErr, localVarHTTPResponse.Status, err.Error())
			return localVarReturnValue, localVarAPIResponse, newErr
		}
		newErr.model = v
		return localVarReturnValue, localVarAPIResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			statusCode: localVarHTTPResponse.StatusCode,
			body:       localVarBody,
			error:      err.Error(),
		}
		return localVarReturnValue, localVarAPIResponse, newErr
	}

	return localVarReturnValue, localVarAPIResponse, nil
}

type ApiRequestsGetRequest struct {
	ctx                 _context.Context
	ApiService          *RequestsApiService
	filters             _neturl.Values
	orderBy             *string
	maxResults          *int32
	pretty              *bool
	depth               *int32
	xContractNumber     *int32
	filterStatus        *string
	filterCreatedAfter  *string
	filterCreatedBefore *string
	filterCreatedDate   *string
	filterCreatedBy     *string
	filterEtag          *string
	filterRequestStatus *string
	filterMethod        *string
	filterHeaders       *string
	filterBody          *string
	filterUrl           *string
	offset              *int32
	limit               *int32
}

func (r ApiRequestsGetRequest) Pretty(pretty bool) ApiRequestsGetRequest {
	r.pretty = &pretty
	return r
}
func (r ApiRequestsGetRequest) Depth(depth int32) ApiRequestsGetRequest {
	r.depth = &depth
	return r
}
func (r ApiRequestsGetRequest) XContractNumber(xContractNumber int32) ApiRequestsGetRequest {
	r.xContractNumber = &xContractNumber
	return r
}
func (r ApiRequestsGetRequest) FilterStatus(filterStatus string) ApiRequestsGetRequest {
	r.filterStatus = &filterStatus
	return r
}
func (r ApiRequestsGetRequest) FilterCreatedAfter(filterCreatedAfter string) ApiRequestsGetRequest {
	r.filterCreatedAfter = &filterCreatedAfter
	return r
}
func (r ApiRequestsGetRequest) FilterCreatedBefore(filterCreatedBefore string) ApiRequestsGetRequest {
	r.filterCreatedBefore = &filterCreatedBefore
	return r
}
func (r ApiRequestsGetRequest) FilterCreatedDate(filterCreatedDate string) ApiRequestsGetRequest {
	r.filterCreatedDate = &filterCreatedDate
	return r
}
func (r ApiRequestsGetRequest) FilterCreatedBy(filterCreatedBy string) ApiRequestsGetRequest {
	r.filterCreatedBy = &filterCreatedBy
	return r
}
func (r ApiRequestsGetRequest) FilterEtag(filterEtag string) ApiRequestsGetRequest {
	r.filterEtag = &filterEtag
	return r
}
func (r ApiRequestsGetRequest) FilterRequestStatus(filterRequestStatus string) ApiRequestsGetRequest {
	r.filterRequestStatus = &filterRequestStatus
	return r
}
func (r ApiRequestsGetRequest) FilterMethod(filterMethod string) ApiRequestsGetRequest {
	r.filterMethod = &filterMethod
	return r
}
func (r ApiRequestsGetRequest) FilterHeaders(filterHeaders string) ApiRequestsGetRequest {
	r.filterHeaders = &filterHeaders
	return r
}
func (r ApiRequestsGetRequest) FilterBody(filterBody string) ApiRequestsGetRequest {
	r.filterBody = &filterBody
	return r
}
func (r ApiRequestsGetRequest) FilterUrl(filterUrl string) ApiRequestsGetRequest {
	r.filterUrl = &filterUrl
	return r
}
func (r ApiRequestsGetRequest) Offset(offset int32) ApiRequestsGetRequest {
	r.offset = &offset
	return r
}
func (r ApiRequestsGetRequest) Limit(limit int32) ApiRequestsGetRequest {
	r.limit = &limit
	return r
}

// Filters query parameters limit results to those containing a matching value for a specific property.
func (r ApiRequestsGetRequest) Filter(key string, value string) ApiRequestsGetRequest {
	filterKey := fmt.Sprintf(FilterQueryParam, key)
	r.filters[filterKey] = append(r.filters[filterKey], value)
	return r
}

// OrderBy query param sorts the results alphanumerically in ascending order based on the specified property.
func (r ApiRequestsGetRequest) OrderBy(orderBy string) ApiRequestsGetRequest {
	r.orderBy = &orderBy
	return r
}

// MaxResults query param limits the number of results returned.
func (r ApiRequestsGetRequest) MaxResults(maxResults int32) ApiRequestsGetRequest {
	r.maxResults = &maxResults
	return r
}

func (r ApiRequestsGetRequest) Execute() (Requests, *APIResponse, error) {
	return r.ApiService.RequestsGetExecute(r)
}

/*
 * RequestsGet List requests
 * List all API requests.
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @return ApiRequestsGetRequest
 */
func (a *RequestsApiService) RequestsGet(ctx _context.Context) ApiRequestsGetRequest {
	return ApiRequestsGetRequest{
		ApiService: a,
		ctx:        ctx,
		filters:    _neturl.Values{},
	}
}

/*
 * Execute executes the request
 * @return Requests
 */
func (a *RequestsApiService) RequestsGetExecute(r ApiRequestsGetRequest) (Requests, *APIResponse, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodGet
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  Requests
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "RequestsApiService.RequestsGet")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/requests"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.pretty != nil {
		localVarQueryParams.Add("pretty", parameterToString(*r.pretty, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("pretty")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("pretty", parameterToString(true, ""))
		}
	}
	if r.depth != nil {
		localVarQueryParams.Add("depth", parameterToString(*r.depth, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("depth")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("depth", parameterToString(0, ""))
		}
	}
	if r.filterStatus != nil {
		localVarQueryParams.Add("filter.status", parameterToString(*r.filterStatus, ""))
	}
	if r.filterCreatedAfter != nil {
		localVarQueryParams.Add("filter.createdAfter", parameterToString(*r.filterCreatedAfter, ""))
	}
	if r.filterCreatedBefore != nil {
		localVarQueryParams.Add("filter.createdBefore", parameterToString(*r.filterCreatedBefore, ""))
	}
	if r.filterCreatedDate != nil {
		localVarQueryParams.Add("filter.createdDate", parameterToString(*r.filterCreatedDate, ""))
	}
	if r.filterCreatedBy != nil {
		localVarQueryParams.Add("filter.createdBy", parameterToString(*r.filterCreatedBy, ""))
	}
	if r.filterEtag != nil {
		localVarQueryParams.Add("filter.etag", parameterToString(*r.filterEtag, ""))
	}
	if r.filterRequestStatus != nil {
		localVarQueryParams.Add("filter.requestStatus", parameterToString(*r.filterRequestStatus, ""))
	}
	if r.filterMethod != nil {
		localVarQueryParams.Add("filter.method", parameterToString(*r.filterMethod, ""))
	}
	if r.filterHeaders != nil {
		localVarQueryParams.Add("filter.headers", parameterToString(*r.filterHeaders, ""))
	}
	if r.filterBody != nil {
		localVarQueryParams.Add("filter.body", parameterToString(*r.filterBody, ""))
	}
	if r.filterUrl != nil {
		localVarQueryParams.Add("filter.url", parameterToString(*r.filterUrl, ""))
	}
	if r.offset != nil {
		localVarQueryParams.Add("offset", parameterToString(*r.offset, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("offset")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("offset", parameterToString(0, ""))
		}
	}
	if r.limit != nil {
		localVarQueryParams.Add("limit", parameterToString(*r.limit, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("limit")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("limit", parameterToString(1000, ""))
		}
	}
	if r.orderBy != nil {
		localVarQueryParams.Add("orderBy", parameterToString(*r.orderBy, ""))
	}
	if r.maxResults != nil {
		localVarQueryParams.Add("maxResults", parameterToString(*r.maxResults, ""))
	}
	if len(r.filters) > 0 {
		for k, v := range r.filters {
			for _, iv := range v {
				localVarQueryParams.Add(k, iv)
			}
		}
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xContractNumber != nil {
		localVarHeaderParams["X-Contract-Number"] = parameterToString(*r.xContractNumber, "")
	}
	if r.ctx != nil {
		// API Key Authentication
		if auth, ok := r.ctx.Value(ContextAPIKeys).(map[string]APIKey); ok {
			if apiKey, ok := auth["Token Authentication"]; ok {
				var key string
				if apiKey.Prefix != "" {
					key = apiKey.Prefix + " " + apiKey.Key
				} else {
					key = apiKey.Key
				}
				localVarHeaderParams["Authorization"] = key
			}
		}
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, httpRequestTime, err := a.client.callAPI(req)

	localVarAPIResponse := &APIResponse{
		Response:    localVarHTTPResponse,
		Method:      localVarHTTPMethod,
		RequestURL:  localVarPath,
		RequestTime: httpRequestTime,
		Operation:   "RequestsGet",
	}

	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarAPIResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarAPIResponse.Payload = localVarBody
	if err != nil {
		return localVarReturnValue, localVarAPIResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			statusCode: localVarHTTPResponse.StatusCode,
			body:       localVarBody,
			error:      fmt.Sprintf(FormatStringErr, localVarHTTPResponse.Status, string(localVarBody)),
		}
		var v Error
		err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
		if err != nil {
			newErr.error = fmt.Sprintf(FormatStringErr, localVarHTTPResponse.Status, err.Error())
			return localVarReturnValue, localVarAPIResponse, newErr
		}
		newErr.model = v
		return localVarReturnValue, localVarAPIResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			statusCode: localVarHTTPResponse.StatusCode,
			body:       localVarBody,
			error:      err.Error(),
		}
		return localVarReturnValue, localVarAPIResponse, newErr
	}

	return localVarReturnValue, localVarAPIResponse, nil
}

type ApiRequestsStatusGetRequest struct {
	ctx             _context.Context
	ApiService      *RequestsApiService
	filters         _neturl.Values
	orderBy         *string
	maxResults      *int32
	requestId       string
	pretty          *bool
	depth           *int32
	xContractNumber *int32
}

func (r ApiRequestsStatusGetRequest) Pretty(pretty bool) ApiRequestsStatusGetRequest {
	r.pretty = &pretty
	return r
}
func (r ApiRequestsStatusGetRequest) Depth(depth int32) ApiRequestsStatusGetRequest {
	r.depth = &depth
	return r
}
func (r ApiRequestsStatusGetRequest) XContractNumber(xContractNumber int32) ApiRequestsStatusGetRequest {
	r.xContractNumber = &xContractNumber
	return r
}

// Filters query parameters limit results to those containing a matching value for a specific property.
func (r ApiRequestsStatusGetRequest) Filter(key string, value string) ApiRequestsStatusGetRequest {
	filterKey := fmt.Sprintf(FilterQueryParam, key)
	r.filters[filterKey] = append(r.filters[filterKey], value)
	return r
}

// OrderBy query param sorts the results alphanumerically in ascending order based on the specified property.
func (r ApiRequestsStatusGetRequest) OrderBy(orderBy string) ApiRequestsStatusGetRequest {
	r.orderBy = &orderBy
	return r
}

// MaxResults query param limits the number of results returned.
func (r ApiRequestsStatusGetRequest) MaxResults(maxResults int32) ApiRequestsStatusGetRequest {
	r.maxResults = &maxResults
	return r
}

func (r ApiRequestsStatusGetRequest) Execute() (RequestStatus, *APIResponse, error) {
	return r.ApiService.RequestsStatusGetExecute(r)
}

/*
 * RequestsStatusGet Retrieve request status
 * Retrieve the status of the specified request.
 * @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param requestId The unique ID of the request.
 * @return ApiRequestsStatusGetRequest
 */
func (a *RequestsApiService) RequestsStatusGet(ctx _context.Context, requestId string) ApiRequestsStatusGetRequest {
	return ApiRequestsStatusGetRequest{
		ApiService: a,
		ctx:        ctx,
		requestId:  requestId,
		filters:    _neturl.Values{},
	}
}

/*
 * Execute executes the request
 * @return RequestStatus
 */
func (a *RequestsApiService) RequestsStatusGetExecute(r ApiRequestsStatusGetRequest) (RequestStatus, *APIResponse, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodGet
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  RequestStatus
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "RequestsApiService.RequestsStatusGet")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/requests/{requestId}/status"
	localVarPath = strings.Replace(localVarPath, "{"+"requestId"+"}", _neturl.PathEscape(parameterToString(r.requestId, "")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}

	if r.pretty != nil {
		localVarQueryParams.Add("pretty", parameterToString(*r.pretty, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("pretty")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("pretty", parameterToString(true, ""))
		}
	}
	if r.depth != nil {
		localVarQueryParams.Add("depth", parameterToString(*r.depth, ""))
	} else {
		defaultQueryParam := a.client.cfg.DefaultQueryParams.Get("depth")
		if defaultQueryParam == "" {
			localVarQueryParams.Add("depth", parameterToString(0, ""))
		}
	}
	if r.orderBy != nil {
		localVarQueryParams.Add("orderBy", parameterToString(*r.orderBy, ""))
	}
	if r.maxResults != nil {
		localVarQueryParams.Add("maxResults", parameterToString(*r.maxResults, ""))
	}
	if len(r.filters) > 0 {
		for k, v := range r.filters {
			for _, iv := range v {
				localVarQueryParams.Add(k, iv)
			}
		}
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	if r.xContractNumber != nil {
		localVarHeaderParams["X-Contract-Number"] = parameterToString(*r.xContractNumber, "")
	}
	if r.ctx != nil {
		// API Key Authentication
		if auth, ok := r.ctx.Value(ContextAPIKeys).(map[string]APIKey); ok {
			if apiKey, ok := auth["Token Authentication"]; ok {
				var key string
				if apiKey.Prefix != "" {
					key = apiKey.Prefix + " " + apiKey.Key
				} else {
					key = apiKey.Key
				}
				localVarHeaderParams["Authorization"] = key
			}
		}
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, httpRequestTime, err := a.client.callAPI(req)

	localVarAPIResponse := &APIResponse{
		Response:    localVarHTTPResponse,
		Method:      localVarHTTPMethod,
		RequestURL:  localVarPath,
		RequestTime: httpRequestTime,
		Operation:   "RequestsStatusGet",
	}

	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarAPIResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarAPIResponse.Payload = localVarBody
	if err != nil {
		return localVarReturnValue, localVarAPIResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			statusCode: localVarHTTPResponse.StatusCode,
			body:       localVarBody,
			error:      fmt.Sprintf(FormatStringErr, localVarHTTPResponse.Status, string(localVarBody)),
		}
		var v Error
		err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
		if err != nil {
			newErr.error = fmt.Sprintf(FormatStringErr, localVarHTTPResponse.Status, err.Error())
			return localVarReturnValue, localVarAPIResponse, newErr
		}
		newErr.model = v
		return localVarReturnValue, localVarAPIResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			statusCode: localVarHTTPResponse.StatusCode,
			body:       localVarBody,
			error:      err.Error(),
		}
		return localVarReturnValue, localVarAPIResponse, newErr
	}

	return localVarReturnValue, localVarAPIResponse, nil
}
