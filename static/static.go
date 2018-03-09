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
#content-0 .cls-toc-r-0 *{color:black}
#content-0 dt{width:8px}
#content-0 dd,#content-0 input.del{width:16px}
#content-0,.header,.footer{margin:0 auto;width:642px}
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

const NewSnippet = `<form method=POST action=/post><table id=post-form>
<tr>
	<td class=title>Title:</td><td><input class=ctrl name=title placeholder="Untitled"></td>
	<td class=title>Your Name:</td><td><input class=ctrl name=author placeholder="N/A"></td>
</tr>
<tr><td colspan=4><textarea class=ctrl name=content rows=10></textarea></td></tr>
<tr><td colspan=4>
Expires in: <select name=ttl>
	<option value="3600">1 hr</option>
	<option value="86400">1 day</option>
	<option value="2592000">30 days</option>
	<option value="0" selected>never</option>
</select> 
<input type=submit value=Post style="float:right">
</td></tr>
</table>
</form>

<hr>

<div style="margin:4px 0">
<ol>
<li>Spaces will be inserted (if needed) into the content to make it fit 80 columns perfectly;
<li>If you don't want spaces messing with your code, use ` + "```" + ` &lt;your code...&gt; ` + "```" + ` to quote them (three backticks); 
<li>URLs automatically become links, image links will be displayed as images (.jpg, .png, .gif and .webp);
<li>Don't post large snippets (though we support size up to 1MB), it will be rendered into tons of HTML elements (~100000) and crash browsers on low-end computers;
<li>You get one cookie after each post, allowing you to delete the snippet in the future;
</ol>
</div>`
