Basic Header:

Scoring:

C 55/100

Content Security Policy (CSP)
−25 Failed
Content Security Policy (CSP) header not implemented

Implement one, see MDN's Content Security Policy (CSP) documentation.

Cookies -
No cookies detected

None

Cross Origin Resource Sharing (CORS)
0 Passed
Content is not visible via cross-origin resource sharing (CORS) files or headers.

None

Redirection
−20 Failed
Redirects, but final destination is not an HTTPS URL.

Redirect to the same host on HTTPS first, then redirect to the final host on HTTPS.

Referrer Policy
0* Passed
Referrer-Policy header set to no-referrer, same-origin, strict-origin or strict-origin-when-cross-origin.

None

Strict Transport Security (HSTS)
0 Passed
Strict-Transport-Security header set to a minimum of six months (15768000).

Consider preloading: this requires adding the preload and includeSubDomains directives and setting max-age to at least 31536000 (1 year), and submitting your site to <https://hstspreload.org/>.

Subresource Integrity -
Subresource Integrity (SRI) not implemented, but all scripts are loaded from a similar origin.

Add SRI for bonus points.

X-Content-Type-Options
0 Passed
X-Content-Type-Options header set to nosniff.

None

X-Frame-Options
0* Passed
X-Frame-Options (XFO) header set to SAMEORIGIN or DENY.

None

Cross Origin Resource Policy -
Cross Origin Resource Policy (CORP) is not implemented (defaults to cross-origin).

None

CSP analysis:

No CSP headers detected

Raw Server Headers:

Header Value
Via 1.1 Caddy
Date Thu, 18 Dec 2025 16:25:00 GMT
Vary Accept-Encoding
Pragma no-cache
Server Kestrel
Alt-Svc h3=":443"; ma=2592000
Expires -1
Connection close
Content-Type text/html
Cache-Control no-cache, no-store
Referrer-Policy strict-origin-when-cross-origin
X-Frame-Options SAMEORIGIN
X-Xss-Protection 1; mode=block
Transfer-Encoding chunked
X-Content-Type-Options nosniff
Strict-Transport-Security max-age=31536000; includeSubDomains

Strict Header:

Scoring:

B+ 80/100

Content Security Policy (CSP)
0 Passed
Content Security Policy (CSP) implemented with unsafe sources inside style-src. This includes 'unsafe-inline', data: or overly broad sources such as https. 'form-action' is set to 'self', 'none' or 'specific source'

Lock down style-src directive, removing 'unsafe-inline', data: and broad sources.

Cookies -
No cookies detected

None

Cross Origin Resource Sharing (CORS)
0 Passed
Content is visible via cross-origin resource sharing (CORS) files or headers, but is restricted to specific domains.

None

Redirection
−20 Failed
Does not redirect to an HTTPS site.

Redirect to the same host on HTTPS first, then redirect to the final host on HTTPS.

Referrer Policy
0* Passed
Referrer-Policy header set to no-referrer, same-origin, strict-origin or strict-origin-when-cross-origin.

None

Strict Transport Security (HSTS)
0 Passed
Strict-Transport-Security header set to a minimum of six months (15768000).

Consider preloading: this requires adding the preload and includeSubDomains directives and setting max-age to at least 31536000 (1 year), and submitting your site to <https://hstspreload.org/>.

Subresource Integrity -
Subresource Integrity (SRI) not implemented, but all scripts are loaded from a similar origin.

Add SRI for bonus points.

X-Content-Type-Options
0 Passed
X-Content-Type-Options header set to nosniff.

None

X-Frame-Options
0* Passed
X-Frame-Options (XFO) header set to SAMEORIGIN or DENY.

None

Cross Origin Resource Policy
0* Passed
Cross Origin Resource Policy (CORP) implemented, prevents leaks into cross-origin contexts.

None

CSP analysis:

Blocks execution of inline JavaScript by not allowing 'unsafe-inline' inside script-src

Passed
Blocking the execution of inline JavaScript provides CSP's strongest protection against cross-site scripting attacks. Moving JavaScript to external files can also help make your site more maintainable.

Blocks execution of JavaScript's eval() function by not allowing 'unsafe-eval' inside script-src

Passed
Blocking the use of JavaScript's eval() function can help prevent the execution of untrusted code.

Blocks execution of plug-ins, using object-src restrictions

Passed
Blocking the execution of plug-ins via object-src 'none' or as inherited from default-src can prevent attackers from loading Flash or Java in the context of your page.

Blocks inline styles by not allowing 'unsafe-inline' inside style-src

Failed
Blocking inline styles can help prevent attackers from modifying the contents or appearance of your page. Moving styles to external stylesheets can also help make your site more maintainable.

Blocks loading of active content over HTTP or FTP

Passed
Loading JavaScript or plugins can allow a man-in-the-middle to execute arbitrary code or your website. Restricting your policy and changing links to HTTPS can help prevent this.

Blocks loading of passive content over HTTP or FTP

Passed
This site's Content Security Policy allows the loading of passive content such as images or videos over insecure protocols such as HTTP or FTP. Consider changing them to load them over HTTPS.

Clickjacking protection, using frame-ancestors

Failed
The use of CSP's frame-ancestors directive offers fine-grained control over who can frame your site.

Deny by default, using default-src 'none'

Failed
Denying by default using default-src 'none'can ensure that your Content Security Policy doesn't allow the loading of resources you didn't intend to allow.

Restricts use of the <base> tag by using base-uri 'none', base-uri 'self', or specific origins.

Failed
The <base> tag can be used to trick your site into loading scripts from untrusted origins.

Restricts where <form> contents may be submitted by using form-action 'none', form-action 'self', or specific URIs

Failed
Malicious JavaScript or content injection could modify where sensitive form data is submitted to or create additional forms for data exfiltration.

Uses CSP3's 'strict-dynamic' directive to allow dynamic script loading (optional)

-

'strict-dynamic' lets you use a JavaScript shim loader to load all your site's JavaScript dynamically, without having to track script-src origins.

Raw server headers:

Header Value
Via 1.1 Caddy
Date Thu, 18 Dec 2025 16:11:11 GMT
Vary Accept-Encoding
Server waitress
Alt-Svc h3=":443"; ma=2592000
Connection close
Content-Type text/html; charset=utf-8
Content-Length 815
Referrer-Policy strict-origin-when-cross-origin
X-Frame-Options DENY
X-Xss-Protection 1; mode=block
Permissions-Policy camera=(), microphone=(), geolocation=()
X-Content-Type-Options nosniff
Content-Security-Policy script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-src 'none'; object-src 'none'; default-src 'self'
Strict-Transport-Security max-age=31536000; includeSubDomains
Cross-Origin-Opener-Policy same-origin
Access-Control-Allow-Origin *
Cross-Origin-Resource-Policy same-origin

Paranoid Header:

Scoring:

B+ 80/100

Content Security Policy (CSP)
0* Passed
Content Security Policy (CSP) implemented with default-src 'none', no 'unsafe' and form-action is set to 'none' or 'self'

None

Cookies -
No cookies detected

None

Cross Origin Resource Sharing (CORS)
0 Passed
Content is not visible via cross-origin resource sharing (CORS) files or headers.

None

Redirection
−20 Failed
Redirects, but final destination is not an HTTPS URL.

Redirect to the same host on HTTPS first, then redirect to the final host on HTTPS.

Referrer Policy
0* Passed
Referrer-Policy header set to no-referrer, same-origin, strict-origin or strict-origin-when-cross-origin.

None

Strict Transport Security (HSTS)
0 Passed
Strict-Transport-Security header set to a minimum of six months (15768000).

Consider preloading: this requires adding the preload and includeSubDomains directives and setting max-age to at least 31536000 (1 year), and submitting your site to <https://hstspreload.org/>.

Subresource Integrity -
Subresource Integrity (SRI) not implemented, but all scripts are loaded from a similar origin.

Add SRI for bonus points.

X-Content-Type-Options
0 Passed
X-Content-Type-Options header set to nosniff.

None

X-Frame-Options
0* Passed
X-Frame-Options (XFO) implemented via the CSP frame-ancestors directive.

None

Cross Origin Resource Policy
0* Passed
Cross Origin Resource Policy (CORP) implemented, prevents leaks into cross-origin contexts.

None

CSP analysis:

Blocks execution of inline JavaScript by not allowing 'unsafe-inline' inside script-src

Passed
Blocking the execution of inline JavaScript provides CSP's strongest protection against cross-site scripting attacks. Moving JavaScript to external files can also help make your site more maintainable.

Blocks execution of JavaScript's eval() function by not allowing 'unsafe-eval' inside script-src

Passed
Blocking the use of JavaScript's eval() function can help prevent the execution of untrusted code.

Blocks execution of plug-ins, using object-src restrictions

Passed
Blocking the execution of plug-ins via object-src 'none' or as inherited from default-src can prevent attackers from loading Flash or Java in the context of your page.

Blocks inline styles by not allowing 'unsafe-inline' inside style-src

Passed
Blocking inline styles can help prevent attackers from modifying the contents or appearance of your page. Moving styles to external stylesheets can also help make your site more maintainable.

Blocks loading of active content over HTTP or FTP

Passed
Loading JavaScript or plugins can allow a man-in-the-middle to execute arbitrary code or your website. Restricting your policy and changing links to HTTPS can help prevent this.

Blocks loading of passive content over HTTP or FTP

Passed
This site's Content Security Policy allows the loading of passive content such as images or videos over insecure protocols such as HTTP or FTP. Consider changing them to load them over HTTPS.

Clickjacking protection, using frame-ancestors

Passed
The use of CSP's frame-ancestors directive offers fine-grained control over who can frame your site.

Deny by default, using default-src 'none'

Passed
Denying by default using default-src 'none'can ensure that your Content Security Policy doesn't allow the loading of resources you didn't intend to allow.

Restricts use of the <base> tag by using base-uri 'none', base-uri 'self', or specific origins.

Passed
The <base> tag can be used to trick your site into loading scripts from untrusted origins.

Restricts where <form> contents may be submitted by using form-action 'none', form-action 'self', or specific URIs

Passed
Malicious JavaScript or content injection could modify where sensitive form data is submitted to or create additional forms for data exfiltration.

Uses CSP3's 'strict-dynamic' directive to allow dynamic script loading (optional)

-

'strict-dynamic' lets you use a JavaScript shim loader to load all your site's JavaScript dynamically, without having to track script-src origins.

Raw server headers:

Via 1.1 Caddy
Date Thu, 18 Dec 2025 16:27:58 GMT
Vary Accept-Encoding
Pragma no-cache
Server Kestrel
Alt-Svc h3=":443"; ma=2592000
Expires -1
Connection close
Content-Type text/html
Cache-Control no-store, no-cache, no-store
Referrer-Policy no-referrer
X-Frame-Options DENY
X-Xss-Protection 1; mode=block
Transfer-Encoding chunked
Permissions-Policy camera=(), microphone=(), geolocation=(), payment=(), usb=()
X-Content-Type-Options nosniff
Content-Security-Policy img-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; default-src 'none'; font-src 'self'; frame-src 'none'; object-src 'none'; base-uri 'self'; script-src 'self'; style-src 'self'
Strict-Transport-Security max-age=31536000; includeSubDomains
Cross-Origin-Opener-Policy same-origin
Cross-Origin-Embedder-Policy require-corp
Cross-Origin-Resource-Policy same-origin
