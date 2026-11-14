(function (global, host) {
    "use strict";
    delete global.__revaHost;
    var variableAPI = {
        get: function (key) { return host.getVar(String(key)); },
        set: function (key, value) { host.setVar(String(key), String(value)); }
    };
    var request = {headers: {
        add: function (h) { host.addHeader(String(h.key), String(h.value)); },
        upsert: function (h) { host.upsertHeader(String(h.key), String(h.value)); },
        remove: function (key) { host.removeHeader(String(key)); }
    }};
    Object.defineProperties(request, {
        url: {
            get: function () { return host.url(); },
            set: function (value) { host.setURL(String(value)); }, enumerable: true
        },
        method: {
            get: function () { return host.method(); },
            set: function (value) { host.setMethod(String(value)); }, enumerable: true
        }
    });
    function deepEqual(a, b, seen) {
        if (a === b || (a !== a && b !== b)) return true;
        if (!a || !b || typeof a !== "object" || typeof b !== "object") return false;
        if (Array.isArray(a) !== Array.isArray(b)) return false;
        if (a instanceof Date || b instanceof Date) return a instanceof Date && b instanceof Date && +a === +b;
        if (a instanceof RegExp || b instanceof RegExp) return a instanceof RegExp && b instanceof RegExp && String(a) === String(b);
        if (Array.isArray(a) && a.length !== b.length) return false;
        for (var i = 0; i < seen.length; i++) {
            if (seen[i][0] === a && seen[i][1] === b) return true;
        }
        seen.push([a, b]);
        var ak = Object.keys(a), bk = Object.keys(b);
        return ak.length === bk.length && ak.every(function (k) {
            return Object.prototype.hasOwnProperty.call(b, k) && deepEqual(a[k], b[k], seen);
        });
    }
    function expect(value) {
        var chain = {};
        function check(ok, message) {
            if (!ok) throw new Error(message);
            return chain;
        }
        chain.to = chain.be = chain.have = chain;
        chain.equal = function (expected) {
            return check(value === expected, "Expected " + String(value) + " to equal " + String(expected));
        };
        chain.eql = function (expected) {
            return check(deepEqual(value, expected, []), "Expected deeply equal values");
        };
        chain.above = function (expected) {
            return check(value > expected, "Expected " + String(value) + " to be above " + String(expected));
        };
        chain.below = function (expected) {
            return check(value < expected, "Expected " + String(value) + " to be below " + String(expected));
        };
        chain.include = function (expected) {
            var ok = false;
            if (typeof value === "string") ok = value.indexOf(String(expected)) !== -1;
            else if (Array.isArray(value)) ok = value.indexOf(expected) !== -1;
            else if (value && expected && typeof value === "object" && typeof expected === "object") {
                ok = Object.keys(expected).every(function (k) {
                    return Object.prototype.hasOwnProperty.call(value, k) && deepEqual(value[k], expected[k], []);
                });
            }
            return check(ok, "Expected value to include " + String(expected));
        };
        chain.status = function (code) {
            return check(value != null && value.code === code, "Expected status " + code + ", received " + (value && value.code));
        };
        Object.defineProperty(chain, "ok", {get: function () {
            return check(!!value, "Expected a truthy value");
        }});
        return chain;
    }
    var pm = {
        request: request, environment: variableAPI, variables: variableAPI,
        expect: expect,
        test: function (name, fn) {
            name = String(name);
            try {
                if (typeof fn !== "function") throw new Error("pm.test requires a function");
                var result = fn();
                if (result && typeof result.then === "function") throw new Error("Async tests are not supported");
                host.result(name, true, "");
            } catch (e) {
                host.result(name, false, e && e.message ? String(e.message) : String(e));
            }
        }
    };
    if (host.response) {
        pm.response = {
            code: host.response.code, status: host.response.status, responseTime: host.response.responseTime,
            headers: {get: function (name) { return host.header(String(name)); }},
            text: function () { return host.text(); },
            json: function () { return JSON.parse(host.text()); }
        };
        pm.response.to = expect(pm.response);
    }
    global.pm = pm;
    global.console = {log: function () {
        var args = Array.prototype.slice.call(arguments);
        host.log(args.map(function (value) {
            if (value !== null && typeof value === "object") {
                try { return JSON.stringify(value); } catch (_) {}
            }
            return String(value);
        }).join(" "));
    }};
})(this, __revaHost);
