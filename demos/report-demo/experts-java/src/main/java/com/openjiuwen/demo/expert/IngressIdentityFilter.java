package com.openjiuwen.demo.expert;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import org.springframework.web.filter.OncePerRequestFilter;

/** Validate the real issuer at the Runtime boundary, including direct query bypasses. */
final class IngressIdentityFilter extends OncePerRequestFilter {
    private final JsonNode config;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build();
    IngressIdentityFilter(JsonNode config) { this.config = config; }

    @Override protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        String path = request.getRequestURI();
        if (!path.equals("/a2a") && !path.equals("/query") && !path.startsWith("/v1/")) { chain.doFilter(request, response); return; }
        String authorization = request.getHeader("x-origin-authorization");
        if (authorization == null) { authorization = request.getHeader("Authorization"); }
        if (authorization == null || !authorization.startsWith("Bearer ")) { response.sendError(401); return; }
        try {
            JsonNode idp = config.path("identity");
            String body = "client_id=" + encode(idp.path("client_id").asText()) + "&client_secret=" + encode(idp.path("client_secret").asText())
                    + "&token=" + encode(authorization.substring(7));
            var call = HttpRequest.newBuilder(URI.create(idp.path("introspection_url").asText())).timeout(Duration.ofSeconds(3))
                    .header("Content-Type", "application/x-www-form-urlencoded").POST(HttpRequest.BodyPublishers.ofString(body)).build();
            var result = http.send(call, HttpResponse.BodyHandlers.ofString());
            JsonNode principal = new ObjectMapper().readTree(result.body());
            if (result.statusCode() != 200 || !principal.path("active").asBoolean()
                    || !principal.path("client_id").asText().equals(idp.path("allowed_client_id").asText())) { response.sendError(403); return; }
            boolean audience=principal.path("aud").isTextual()&&principal.path("aud").asText().equals(idp.path("audience").asText());
            for(JsonNode value:principal.path("aud")){if(value.asText().equals(idp.path("audience").asText())){audience=true;}}
            if(!audience||!principal.path("iss").asText().equals(idp.path("issuer").asText())
                    ||!principal.path("sub").asText().equals(request.getHeader("x-user-id"))){response.sendError(403);return;}
            chain.doFilter(request, response);
        } catch (Exception e) { response.sendError(503, "Identity verification unavailable"); }
    }
    private static String encode(String value) { return URLEncoder.encode(value, StandardCharsets.UTF_8); }
}
