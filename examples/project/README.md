# project

Command project is the tp-micro service project.
<br>The framework reference: https://github.com/xiaoenai/tp-micro

## API Desc

### Stat

Stat handler

- URI: `/project/stat`
- REQ-QUERY:
	- `ts={int64}`	// timestamps
- REQ-BODY:

	```js
	{}
	```

- RESULT:


### Home

Home handler

- URI: `/project/home`
- REQ-QUERY:
- REQ-BODY:
- RESULT:

	```js
	{
		"content": ""	// {string} // text
	}
	```



### Math_Divide

Divide handler

- URI: `/project/math/divide`
- REQ-QUERY:
- REQ-BODY:

	```js
	{
		"a": -0.000000,	// {float64} // dividend
		"b": -0.000000	// {float64} // divisor
	}
	```

- RESULT:

	```js
	{
		"c": -0.000000	// {float64} // quotient
	}
	```





<br>

*This is a project created by `micro gen` command.*

*[About Micro Command](https://github.com/xiaoenai/tp-micro/tree/v2/cmd/micro)*
