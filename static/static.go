package static

const CSS = `<style>
*{box-sizing:border-box;margin:0}
body{font-size:14px;font-family:Arial,Helvetica,sans-serif}
#content-0{padding:2px 0}
#content-0 div{padding:1px 0;min-height:1em;margin:0;max-height:280px;width:100%;line-height:1.5}
#content-0 dl.err{color:red}
#content-0 dl,
#content-0 dt,
#content-0 dd{display:inline-block;zoom:1;*display:inline;white-space:pre;padding:0;margin:0;font-family:consolas,monospace;text-align:center;text-decoration:none}
#content-0 a{border-bottom: solid 1px;cursor:pointer;text-decoration:none}
#content-0 a:hover{background-color:#ffa}
#content-0 dt.conj{-webkit-touch-callout:none;-webkit-user-select:none;-khtml-user-select:none;-moz-user-select:none;-ms-user-select:none;user-select:none;color:#ccc}
#content-0 ._image{width:auto;max-width:100%;max-height:280px}
#content-0 .cls-toc-r *{color:black}
#content-0 dt{width:8px}
#content-0 dd,#content-0 input.del{width:16px}
#content-0,.header,.footer{margin:0 auto;width:642px}
#post-form{margin:4px 0}
#post-form td{padding:2px;}
#post-form, #post-form .ctrl{resize:vertical;width:100%;max-width:100%;min-width:100%}
#post-form .title{white-space:nowrap;width:1px;text-align:right}
.header a,
.footer a,
.header span,
.footer span{color:white;text-decoration:none;display:inline-block;zoom:1;*display:inline;padding:0 2px}
.header .sep,.footer .sep{color:#ccc}
.header a:hover,.footer a:hover{text-decoration:underline}
.header,.footer{background:#667;padding:4px;color:white}
</style>`

const UntitledSnippet = "无标题"

const NewSnippet = "新片段"

const AllSnippets = "浏览"

const NoPermission = "无权限删除改片段"

const SnippetNotFound = "未找到该片段，其可能已被删除或失效"

const InternalError = "内部错误"

const EmptyContent = "无法提交空内容"

const Back = "后退"

const Delete = "删除"

const Error = "错误"

const NewSnippetForm = `<form method=POST action=/post><table id=post-form>
<tr>
	<td class=title>标题:</td><td><input class=ctrl name=title placeholder="` + UntitledSnippet + `"></td>
	<td class=title>发布者:</td><td><input class=ctrl name=author placeholder="N/A"></td>
	<td class=title>有效期:</td><td class=title><select name=ttl>
		<option value="3600">1小时</option>
		<option value="86400">1天</option>
		<option value="2592000">30天</option>
		<option value="0" selected>永久</option>
	</select></td>	
</tr>
<tr><td colspan=6><textarea class=ctrl name=content rows=10></textarea></td></tr>
<tr><td colspan=6 style="text-align:center"><input type=submit value="发布"></td></tr>
</table>
</form>
<div style="margin:4px 0;font-size:0.9em">
<ol>
<li>每行文本可能会被插入多个空格以保证与80列对齐，若不想被空格破坏格式（如代码），请使用一对三反引号包起：` + "```" + ` &lt;your code...&gt; ` + "```" + `；
<li>段落标题以4个井号（####）开头，发布后自动生成目录，4个等号（====）单独一行转换为分割线；
<li>请不要发布过长的文本（虽然最高允许1MB），其渲染结果可能包含数十万个HTML标签，导致老旧的浏览器直接崩溃；
</ol>
</div>`
