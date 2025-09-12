# Problem

The same post might be posted to a different groups. Thus we'll have duplicates in our destinations. 
Help me designing how this can be alleviated. 

My approach:

We should create a new table `posts_duplicates`, where we are going to store posts for which duplicates already exist. 

``` SQL
create table if not exists posts_duplicates (
    owner_id, post_id REFERENCES posts(....),

    text TEXT, 

    type ,
    animala ,
    sex ,
    phones ,

)
```

For every new post we are going to check this table first, probably using metadata first. 

If no duplicate is found, we are checking original `posts` table, also using metadata first. 

Search should be limited to recent posts only (no older than one week).

Also I have to figure out is to how effeciently check that the post is the same, I guess I need some kind of fuzzy search. 
And maybe compare not the whole `text` fields, but substr of it. 

