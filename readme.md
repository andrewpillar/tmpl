# tmpl

tmpl is a simple utility for templating text files. This uses Go's
[text/template](https://golang.org/pkg/text/template/) library for templating
information into a file. All data templated in files via tmpl is treated as
plain text.

Assume we have a text file with the following content,

    $ cat hello.tmpl
    Hello, {{.Name}}

we would use tmpl like so on the file,

    $ tmpl -var Name=Andrew hello.tmpl
    Hello, Andrew

tmpl will print out the template file by default. The `-var` flag can be passed
multiple times to define multiple variables,

    $ cat hello.tmpl
    Hello, {{.Name1}}

    Goodbye, {{.Name2}}
    $ tmpl -var Name1=me -var Name2=you hello.tmpl
    Hello, me

    Goodbye, you

there may be cases where you have lots of variables to template into a single
file. In instances like this it would be impractical to passthrough lots of
`-var` flags, so tmpl allows you to specify a variable file via the `-file`
flag. The variable file is a simple plain text file that defines variables on
separate lines.

    $ cat article.vars
    # Comments are lines prefixed with the '#' character.
    Title  = Document title
    Author = Andrew
    $ cat article.tmpl
    {{.Title}}
    written by {{.Author}} on {{.Date}}
    $ tmpl -file article.vars -var Date="$(date +"%F")"
    Document title
    written by Andrew on 2006-01-02
