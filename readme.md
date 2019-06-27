# WebAPI

```
 _       ____  ___    __    ___   _  
\ \    /| |_  | |_)  / /\  | |_) | | 
 \_\/\/ |_|__ |_|_) /_/--\ |_|   |_| 
 
```

[中文](https://github.com/go-webapi/webapi/blob/master/readme.zh-cn.md) | English

WebAPI is a basic library for Web API service development for Golang. Using WebAPI can effectively reduce the probability of coding errors and avoid boring duplicate code. WebAPI for Golang is inspired by Microsoft ASP.NET Core, so if you are familiar with .NET or JavaEE developers, you will be well equipped.

## What & How

### Auto-Registration & Composite Controller

**Focusing on business core avoidance of irrelevant rules, DDD is highly compatible with design patterns such as MVVM. Different business modules have their own controllers (groups), which support the design of module subdivision and multi-module unified integration (allowing multi-person collaboration). Conventional automatic route registration reduces redundant code, improves design efficiency while avoiding the possibility of loss of synchronization between the public address and the internal implementation.**

Declare the controller:

```go
type Article struct {
	webapi.Controller
}
```

Declare the endpoint:

```go
func (article *Article) Show(query struct {
  GUID string `json:"guid"`
}) string {
	return fmt.Sprintf("you are reading post-%s", query.GUID)
}
```

> WebAPI follows the Golang principle, any accessible method (a function that begins with an uppercase letter) is registered as an API endpoint.

The endpoint will be registered as `/article/show?guid=[guid]`. If there are multiple controllers handling different services, the controller alias can be specified by the `RouteAlias() string` method:

```go
type article struct {
	webapi.Controller
	id uint
}

func (article *article) RouteAlias() string {
	return "article"
}
```

Then both methods from `article` and `Article` will be registered under `/article`. However, it should be noted that such registrars need to avoid duplicate names. However, at runtime the WebAPI wo**n't** make a fatal warning(panic) for the duplicate name method.

> When you use it, you can divide each business module into different people to complete. Finally, they can be easily integrated through `RouteAlias`.

### Query/Body Auto-Serialization Support

**Complete elimination of text reading, serialization/deserialization, query retrieval and transformation in business logic, and even middleware, you can even configure non-system built-in/private sequencers such as MsgPack.**

Try to use `curl` to access the API just now:

```bash
~ curl http://localhost:9527/article/show\?guid\=79526
#you are reading post-79526
```

You can see that the parameters are automatically placed in `query`. It also supports automatic text serialization, such as declaration methods:

```go
func (article *article) Save(entity *struct {
	ID         uint
	Title      string
	Content    string
	CreateTime time.Time
}, query struct {
	CreateTime string `json:"time"`
}) {
	entity.CreateTime, _ = time.Parse("2006-01-02", query.CreateTime)
	entity.ID = article.id
	article.Reply(http.StatusAccepted, entity)
}
```

This method will be registered as `[POST] /article/{digits}/save`, because there is a `*struct{}` structure, so the default method is POST. However, the HTTP methods can be explicitly declared in the form of a private field of `method[HTTPMETHOD] struct`. Also use `curl`:

```go
~ curl -X "POST" "http://localhost:9527/article/123/save?time=2019-01-01" \
     -H 'Content-Type: application/json; charset=utf-8' \
     -d $'{
  "Title": "Hello WebAPI for Golang",
  "Content": "Awesome!"
}'

#[{"ID":123,"Title":"Hello WebAPI for Golang","Content":"Awesome!","CreateTime":"2019-01-01T00:00:00Z"}]
```

You can see that the time in the query was successfully accessed and assigned to the body. Please also note that the previously used `string` as the return value, but this node manually reply to the client using `.Reply(STATUSCODE, INTERFACE{})` to automatically process object.

> The serializer can be specified manually with Serializer property in the Context.

At the same time, the query and body structure support check, add the `Check() error` method to them to check the legality of the data before entering the business code, and isolate the defense code from the business.

### Routing Precondition (Pre-Parameterized Access) Support

**The convergence controller handles the data category and sets up API access barriers. Provides matching-fallback and specific service controller pre-conditions that other routing services cannot provide, which isolates illegal access, reduces the probability of errors, improves service coding efficiency, and improves system robustness.**

As you saw in the previous request, `123` in the access address `/article/123/save` was caught and finally appeared in the `ID` of the reply body. WebAPI allows preconditions (Precondition) to be set for the controller, the declared method:

```go
func (article *article) Init(id uint) (err error) {
	article.id = id
	return
}
```

You only need to declare a method with a return value of `error` and the name `Init` for the controller to automatically call it before entering the actual method. There have also been some changes to the node registration form. If the parameter is

- Integer, long integer, unsigned integer, unsigned long integer (Int/UInt) will get a registration point for `/{digits}`
- Single precision floating point, double precision floating point (Float32/64) then will get a registration point of `/{float}`
- String will get a registration point for `/{string}`

So the above `Init(uint) error` function will generate a registration point for `/{digits}`.

If the error value returned by the calling function is not empty, the client Bad Request will be notified. This distinguishes between the Object Method and the Static Method from the side. When coding, you can pay more attention to the business itself without worrying about various preconditions, or saving a lot of approximate code.

### Endpoint Condition (Post-Parameterized Access) Support

**No reverse proxy required to configure pseudo-static to provide native support for parameterized access, providing a more intuitive and concise API.**

Since the pre-parameterized access is supported, then natural parameterized access is also possible. Just now we encountered that access to the body of the article requires the use of query parameters, although it can work normally, but it seems too monotonous. Supporting the use of pre-parameters requires a method like `Read`, which feels unnatural. We can provide access forms like `/article/{guid}` in the form of post-parameters:

```go
func (article *Article) Index(guid string) string {
	return fmt.Sprintf("you are reading post-%s", guid)
}
```

Test with `curl`:

```go
~ curl http://localhost:9527/article/id-233666
#you are reading post-id-233666
```

> ⚠️ Attention
>
> This method can also be implemented by the method of `func (article *article) Index(id int)`. In this case, the two methods allow coexistence because the former is `/article/{string}` and the latter is `/article/{digits}`. Be aware of such issues when collaborating. If a duplicate registration node occurs, the controller will fail to register and prompt an error.

### Midway Encryption Policy Support

**Fully entrusted native encryption and decryption, compatible with key negotiation mechanism, even if the key is changed or not unique, only one setting, the pipeline plaintext ciphertext processing is more reliable, completely eliminate the possibility of confidential metadata leakage caused by negligence or accident Data security is safe and worry-free.**

The encryption and decryption service relies on the context, and can specify the `CryptoService` in the context in the middleware to provide a unified encryption and decryption service, and to separate the encryption and decryption methods of the unrelated business, thereby improving the developer efficiency.

The method can also be changed in the middle of use, that is, the module for encryption and decryption is dynamically replaceable.

## Performance

In a test environment of 8 vCPU / 16G RAM TLinux virtual machine (not idle) via the Cyborg performance utility from 100 clients, 200 requests to initiate pressure to 580 client, 1160 requested Hello World interface performance record:

| Clients | Requests | Total | QPS |
| -------- | ------ | ---------- | ---------- |
| 100      | 200    | 0.011942s  | 8374.09354 |
| 120      | 240    | 0.010043s  | 11949.1254 |
| 140      | 280    | 0.008378s  | 16710.2346 |
| 160      | 320    | 0.012892s  | 12410.937  |
| 180      | 360    | 0.011250s  | 15999.7355 |
| 200      | 400    | 0.011609s  | 17228.3244 |
| 220      | 440    | 0.017484s  | 12582.8826 |
| 240      | 480    | 0.020186s  | 11889.3559 |
| 260      | 520    | 1.004284s  | 258.890958 |
| 280      | 560    | 0.015453s  | 18119.6865 |
| 300      | 600    | 0.016319s  | 18383.022  |
| 320      | 640    | 1.011559s  | 316.343278 |
| 340      | 680    | 1.014014s  | 335.301017 |
| 360      | 720    | 0.016673s  | 21591.3134 |
| 380      | 760    | 0.020759s  | 18305.5029 |
| 400      | 800    | 1.007692s  | 396.946831 |
| 420      | 840    | 0.024735s  | 16980.1533 |
| 440      | 880    | 0.023144s  | 19011.1727 |
| 460      | 920    | 0.025437s  | 18083.5409 |
| 480      | 960    | 1.006551s  | 476.875941 |
| 500      | 1000   | 1.008704s  | 495.685546 |
| 520      | 1040   | 1.010670s  | 514.510124 |
| 540      | 1080   | 1.009176s  | 535.089889 |
| 560      | 1120   | 0.034008s  | 16466.9452 |
| 580      | 1160   | 0.031361s  | 18494.0912 |

Since the target machine and the pressure-initiating machine are both virtual machines, data jitter is presented. According to actual experience, this jitter does not exist on the physical server. The median overall performance was 12582.88259 (1.2w), and the median recording performance below 1000 was discarded. 16980.15331 (1.7w). Performance is relatively optimistic for a single node.

---

Finally, Welcome use WebAPI. Cool to Code.