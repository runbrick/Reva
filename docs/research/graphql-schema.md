# Browse GraphQL root fields

In a GraphQL request’s **Body** tab, choose **Load schema** to send an introspection POST to the current endpoint. The browser shows the root Query, Mutation, and Subscription fields with their return types. **Filter root fields** matches root kinds, field names, and types without regard to case.

The load uses the current variables, headers, and resolved authentication. Reva stops reading at the 512 KiB response limit and caps the load at 30 seconds. The request goes to the selected endpoint directly. **Cancel schema load** stops the request, and changing the endpoint clears its results. Schema results stay in memory. The saved operation and scripts still run through the regular **Send** action.
