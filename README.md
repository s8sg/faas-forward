## faas-forward
Chain OpenFaaS function

```go
import github.com/s8sg/faas/forward

// Create a reusable chain
chain := forward.NewFuncChain().apply(“resize_image”, nil).apply(“color_image”, nil).apply(“add_saturation”, nil)
err = chain.Deploy()

// Invoke the chain
var io.ReadCloser result;
var err error;
result, err = chain.Invoke((io.Reader)image_file)



// Async invoke chain
chain = forward.NewFuncChain().apply(“resize_image”, nil).apply(“color_image”, nil).apply(“add_saturation”, nil).async_apply(“upload_to_storage”, map[string]string {.”url”, “http://file-storage:8080” })
err = chain.Deploy()
// Result is empty array if async reply
_, err = chain.Invoke((io.Reader)image_file)
```

### Getting Started
> Currently the automated chain creation is not implemented although you can do it manually by specifying a stack to chain functions

#### Manual

**Pull the `faas-forward` template**
> ```go
> faas-cli template pull https://github.com/s8sg/faas-forward
> ```
> Currently only go template is available as `forward-go`
    
**Create a new function**     
> ```bash
> faas-cli new --lang forward-go myfunc
> ```
     
**Implement the function**
> edit `myfunc/handle.go`  
> ```bash
> // Handle a serverless request
> func Handle(req []byte) ([]byte, error) {
>        return []byte(fmt.Sprintf("Hello, Go-Forward: %s", string(req))), nil
> }
> ```
   
**Define stack.yml**
> rename `myfunc.yml` to `stack.yml`   
> define `stack.yml`   
>```yaml
> provider:
>  name: faas
>  gateway: http://127.0.0.1:8080
>
> # Stack equivalent of data.apply("myfunc1", nil).apply(“myfunc2”, nil).apply(“myfunc3”, nil)
> functions:
>   myfunc1:
>    lang: forward-go
>    handler: ./myfunc
>    image: s8sg/myfunc:latest
>    environment:
>        input_type: "POST" // handle a POST request (also specify the begining of chain)
>        async: false
>        forward: myfunc2
>
>   myfunc2:
>    lang: forward-go
>    handler: ./myfunc
>    image: s8sg/myfunc:latest
>    environment:
>        # Default input type for forward function is multipart file (DATA)
>        input_type: "DATA"
>        # async allows to execute the nexy function in async manner
>        async: false
>        forward: myfunc3
>
>   myfunc3:
>    lang: forward-go
>    handler: ./myfunc
>    image: s8sg/myfunc:latest
>    environment:
>        # Default input type for forward function is multipart file (DATA)
>        input_type: "DATA"
>        # No forward defines the end of a chain
>```
