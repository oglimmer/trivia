# goal

we need to build a trivia , mobile first app.

# app functionality

## landing page

the landing page asks for a gamecode, e.g. id5 or e3fx

also a link to the admin section. 

## regular users

a game has 2 modes, setup mode and game mode

## setup mode

in setup mode everybody can add/update 1 item. each user has to give his/her name. and can upload a personal image. this will be used when user lists are shown.

the user needs to upload a photo, keep in mind that most people will use a mobile device
then add a question for this photo.
a question has a question text and answers.
answers can be YES/NO; 2,3,4 predefined answers; number to guess
the user has to define the right and wrong answers.
there is a button to help with claude AI API to setup the question and / or answers

one in game mode, the game is using a random order of question. and asks one by one. where the admin has to switch to a new question.

scoring:
come up with a sophisticated but fair scoring
taking into account :time to give an answer
the more answers the more points as the question was more difficult

## game mode

in game mode, after each question show the right answer and the people who were right and the time they needed with their scores

after all questions are done, the final scores are shown. starting with 3rd then 2nd then 1st position. each on a different page. after accepting the winner, everybody sees where he/she in a score ladder.

# in admin mode

an admin has to authenticate

an admin has a special area /admin
you can created game events
each game event has a state setup or game
in setup, the admin sees all questions created by the users.
in game mode. the admin has control on activating a question. finishing a question. moving next. there is a timer shown for the admin to understand how long a question is active.

# technically
- we need a frontend in vue and backend in golang.
- the frontend needs to use websockets as the user should never need to reload/refresh the page
- database is postgres
