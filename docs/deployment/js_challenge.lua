-- JS challenge for nginx access phase.
--
-- Opt-in: only active when $js_challenge_enabled is set to "1".
-- On first request (no valid cookie), serves a page with inline JS that
-- sets an HMAC-signed cookie and reloads. The cookie is bound to the
-- client IP and expires after TTL seconds.
--
-- Cookie format: base64(timestamp:base64(HMAC-SHA1(ip:timestamp, secret)))

local TTL = 86400
local secret

-- Read the HMAC secret from disk, cached after first call.
local function get_secret()
	if secret then return secret end
	local f, err = io.open("/etc/nginx/lua/.secret", "r")
	if not f then
		ngx.log(ngx.ERR, "js_challenge: ", err)
		return nil
	end
	secret = f:read("*l")
	f:close()
	if not secret or secret == "" then
		ngx.log(ngx.ERR, "js_challenge: empty secret")
		return nil
	end
	return secret
end

-- HMAC-SHA1 of client IP and timestamp.
local function sign(s, ts)
	return ngx.encode_base64(ngx.hmac_sha1(s, ngx.var.remote_addr .. ":" .. ts))
end

-- Check whether the _js_ok cookie is present, not expired, and valid.
local function validate(s)
	local raw = ngx.decode_base64(ngx.var.cookie__js_ok or "")
	if not raw then return false end
	local ts, mac = raw:match("^(%d+):(.+)$")
	if not ts then return false end
	if ngx.time() - tonumber(ts) > TTL then return false end
	return mac == sign(s, ts)
end

-- Serve a challenge page. The cookie value is obfuscated as a char code
-- array so that a non-JS HTTP client cannot extract it.
local function challenge(s)
	local ts = tostring(ngx.time())
	local val = ngx.encode_base64(ts .. ":" .. sign(s, ts))
	local codes = {}
	for i = 1, #val do codes[i] = string.byte(val, i) end
	ngx.header.content_type = "text/html; charset=utf-8"
	ngx.header.cache_control = "no-store"
	ngx.say(string.format( [=[<!doctype html>
<html><head><title>Verifying</title></head><body>
<noscript><p>Enable JavaScript to access this site.</p></noscript><script>
(function(){
	var a = [%s], s = "";
	for(var i = 0; i < a.length; i++)
		s += String.fromCharCode(a[i]);
		document.cookie = "_js_ok=" + s + ";path=/;max-age=%d;SameSite=Lax;Secure";
		location.reload();
	})()
</script></body></html>]=], table.concat(codes, ","), TTL))
	ngx.exit(200)
end

local e = ngx.var.js_challenge_enabled
if not e or e == "" or e == "0" then return end

local s = get_secret()
if s and not validate(s) then challenge(s) end
